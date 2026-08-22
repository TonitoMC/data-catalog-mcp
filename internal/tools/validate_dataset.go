package tools

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
)

// ValidationIssue is one rule that didn't hold. Column is empty for
// dataset-level rules (e.g. row_count).
type ValidationIssue struct {
	Column   string `json:"column,omitempty"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ValidationResult is the validate_dataset outcome: Valid is false iff at
// least one "error"-severity issue was found ("warning" issues are
// reported but don't fail the dataset).
type ValidationResult struct {
	Dataset  string             `json:"dataset"`
	RowCount int64              `json:"row_count"`
	Valid    bool               `json:"valid"`
	Issues   []ValidationIssue  `json:"issues"`
}

// ValidateDataset checks a dataset's actual Parquet data against every
// column- and dataset-level rule declared for it in the catalog.
func ValidateDataset(cat *catalog.Catalog, dataDir, dataset string) (*ValidationResult, error) {
	ds, ok := cat.Dataset(dataset)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDatasetNotFound, dataset)
	}

	rowCount, err := data.RowCount(dataDir, ds.File)
	if err != nil {
		return nil, fmt.Errorf("tools: validate dataset %s: %w", dataset, err)
	}

	var issues []ValidationIssue

	for _, rule := range ds.DatasetRules {
		if iss := checkDatasetRule(rule, rowCount); iss != nil {
			issues = append(issues, *iss)
		}
	}

	for _, col := range ds.Columns {
		if len(col.Rules) == 0 {
			continue
		}
		cells, err := data.ReadColumn(dataDir, ds.File, col.Name)
		if err != nil {
			issues = append(issues, ValidationIssue{
				Column: col.Name, Rule: "read", Severity: "error",
				Message: "could not read column: " + err.Error(),
			})
			continue
		}
		for _, rule := range col.Rules {
			if iss := checkColumnRule(col.Name, rule, cells); iss != nil {
				issues = append(issues, *iss)
			}
		}
	}

	valid := true
	for _, iss := range issues {
		if iss.Severity == "error" {
			valid = false
			break
		}
	}

	return &ValidationResult{Dataset: dataset, RowCount: rowCount, Valid: valid, Issues: issues}, nil
}

// checkDatasetRule evaluates one dataset-level rule.
func checkDatasetRule(rule catalog.Rule, rowCount int64) *ValidationIssue {
	switch rule.Kind {
	case "row_count":
		constraints, ok := rule.Args["value"].(map[string]any)
		if !ok {
			return unsupportedRule(rule.Kind)
		}
		if min, ok := numOf(constraints["min"]); ok && rowCount < int64(min) {
			return &ValidationIssue{Rule: rule.Kind, Severity: rule.Severity,
				Message: fmt.Sprintf("row count %d is below minimum %d", rowCount, int64(min))}
		}
		if max, ok := numOf(constraints["max"]); ok && rowCount > int64(max) {
			return &ValidationIssue{Rule: rule.Kind, Severity: rule.Severity,
				Message: fmt.Sprintf("row count %d is above maximum %d", rowCount, int64(max))}
		}
		return nil
	default:
		return unsupportedRule(rule.Kind)
	}
}

// checkColumnRule evaluates one column-level rule against every value read
// from that column.
func checkColumnRule(column string, rule catalog.Rule, cells []data.Cell) *ValidationIssue {
	fail := func(format string, args ...any) *ValidationIssue {
		return &ValidationIssue{Column: column, Rule: rule.Kind, Severity: rule.Severity, Message: fmt.Sprintf(format, args...)}
	}

	switch rule.Kind {
	case "not_null":
		n := 0
		for _, c := range cells {
			if c.Null {
				n++
			}
		}
		if n > 0 {
			return fail("%d null value(s)", n)
		}

	case "unique":
		seen := make(map[string]int)
		for _, c := range cells {
			if !c.Null {
				seen[fmt.Sprint(c.Value)]++
			}
		}
		dupes := 0
		for _, n := range seen {
			if n > 1 {
				dupes++
			}
		}
		if dupes > 0 {
			return fail("%d duplicated value(s)", dupes)
		}

	case "range":
		bounds, ok := rule.Args["value"].([]any)
		if !ok || len(bounds) != 2 {
			return unsupportedRuleFor(column, rule.Kind)
		}
		min, ok1 := numOf(bounds[0])
		max, ok2 := numOf(bounds[1])
		if !ok1 || !ok2 {
			return unsupportedRuleFor(column, rule.Kind)
		}
		violations := 0
		for _, c := range cells {
			if c.Null {
				continue
			}
			f, ok := toFloat(c.Value)
			if ok && (f < min || f > max) {
				violations++
			}
		}
		if violations > 0 {
			return fail("%d value(s) outside range [%v, %v]", violations, min, max)
		}

	case "allowed_values":
		allowed, ok := rule.Args["value"].([]any)
		if !ok {
			return unsupportedRuleFor(column, rule.Kind)
		}
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, a := range allowed {
			allowedSet[fmt.Sprint(a)] = struct{}{}
		}
		violations := 0
		for _, c := range cells {
			if c.Null {
				continue
			}
			if _, ok := allowedSet[fmt.Sprint(c.Value)]; !ok {
				violations++
			}
		}
		if violations > 0 {
			return fail("%d value(s) outside the allowed set", violations)
		}

	case "regex":
		pattern, ok := rule.Args["value"].(string)
		if !ok {
			return unsupportedRuleFor(column, rule.Kind)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return unsupportedRuleFor(column, rule.Kind)
		}
		violations := 0
		for _, c := range cells {
			if c.Null {
				continue
			}
			if !re.MatchString(fmt.Sprint(c.Value)) {
				violations++
			}
		}
		if violations > 0 {
			return fail("%d value(s) failed to match %q", violations, pattern)
		}

	case "type_check":
		expected, ok := rule.Args["value"].(string)
		if !ok {
			return unsupportedRuleFor(column, rule.Kind)
		}
		violations := 0
		for _, c := range cells {
			if c.Null {
				continue
			}
			if !matchesType(c.Value, expected) {
				violations++
			}
		}
		if violations > 0 {
			return fail("%d value(s) not valid %s", violations, expected)
		}

	default:
		return unsupportedRuleFor(column, rule.Kind)
	}
	return nil
}

// matchesType reports whether v (already a bool/int64/float64/string, per
// data.Cell) satisfies the catalog-declared type name. Values stored as
// strings are parsed, so a "dirty" extract with e.g. "N/A" in a float
// column is correctly flagged.
func matchesType(v any, expected string) bool {
	s := fmt.Sprint(v)
	switch expected {
	case "float":
		_, err := strconv.ParseFloat(s, 64)
		return err == nil
	case "int", "integer":
		_, err := strconv.ParseInt(s, 10, 64)
		return err == nil
	case "bool", "boolean":
		_, err := strconv.ParseBool(s)
		return err == nil
	case "string":
		return true
	default:
		return true // unknown declared type: nothing to check against
	}
}

// numOf converts a YAML-decoded numeric value (int or float64, depending
// on how it was written) to float64.
func numOf(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}

// unsupportedRule reports a dataset-level rule this server doesn't know
// how to check yet (e.g. freshness), as a non-blocking warning rather than
// failing validation outright.
func unsupportedRule(kind string) *ValidationIssue {
	return &ValidationIssue{Rule: kind, Severity: "warning", Message: "rule kind not implemented, skipped"}
}

func unsupportedRuleFor(column, kind string) *ValidationIssue {
	return &ValidationIssue{Column: column, Rule: kind, Severity: "warning", Message: "rule kind not implemented or malformed, skipped"}
}
