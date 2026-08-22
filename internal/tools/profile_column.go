package tools

import (
	"errors"
	"fmt"
	"sort"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
)

// ColumnProfile summarizes the actual values found in one column: how many
// there are, how many are missing, how many distinct values, and (for
// numeric columns) the observed range and mean.
type ColumnProfile struct {
	Dataset       string       `json:"dataset"`
	Column        string       `json:"column"`
	Type          string       `json:"type,omitempty"` // catalog-declared type; empty if undocumented
	Count         int64        `json:"count"`
	NullCount     int64        `json:"null_count"`
	NullRatio     float64      `json:"null_ratio"`
	DistinctCount int64        `json:"distinct_count"`
	Min           *float64     `json:"min,omitempty"`
	Max           *float64     `json:"max,omitempty"`
	Mean          *float64     `json:"mean,omitempty"`
	ValueCounts   []ValueCount `json:"value_counts,omitempty"`  // full frequency table, low-cardinality columns only
	SampleValues  []string     `json:"sample_values,omitempty"` // a few examples, high-cardinality columns only
}

// ValueCount is one distinct value and how many times it occurs.
type ValueCount struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// maxValueCounts bounds cardinality for returning a full frequency table
// (e.g. Yes/No, or a handful of categories) rather than a sample: above
// this many distinct values, a full table is more noise than signal.
const maxValueCounts = 20

// maxSampleValues bounds how many example values a high-cardinality
// column's profile carries.
const maxSampleValues = 5

// ProfileColumn reads every value of column in dataset and computes
// summary statistics.
func ProfileColumn(cat *catalog.Catalog, dataDir, dataset, column string) (*ColumnProfile, error) {
	ds, ok := cat.Dataset(dataset)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDatasetNotFound, dataset)
	}

	cells, err := data.ReadColumn(dataDir, ds.File, column)
	if err != nil {
		if errors.Is(err, data.ErrColumnNotFound) {
			return nil, fmt.Errorf("%w: %s.%s", ErrColumnNotFound, dataset, column)
		}
		return nil, fmt.Errorf("tools: profile column %s.%s: %w", dataset, column, err)
	}

	profile := &ColumnProfile{Dataset: dataset, Column: column, Count: int64(len(cells))}
	if col, ok := findColumn(ds, column); ok {
		profile.Type = col.Type
	}

	counts := make(map[string]int64)
	var min, max, sum float64
	haveNumeric := false
	numericCount := int64(0)

	for _, c := range cells {
		if c.Null {
			profile.NullCount++
			continue
		}

		key := fmt.Sprint(c.Value)
		counts[key]++

		if f, ok := toFloat(c.Value); ok {
			if !haveNumeric || f < min {
				min = f
			}
			if !haveNumeric || f > max {
				max = f
			}
			sum += f
			numericCount++
			haveNumeric = true
		}
	}

	profile.DistinctCount = int64(len(counts))
	if profile.Count > 0 {
		profile.NullRatio = float64(profile.NullCount) / float64(profile.Count)
	}
	if haveNumeric {
		profile.Min = &min
		profile.Max = &max
		mean := sum / float64(numericCount)
		profile.Mean = &mean
	}

	if int64(len(counts)) <= maxValueCounts {
		profile.ValueCounts = sortedValueCounts(counts)
	} else {
		profile.SampleValues = sampleKeys(counts, maxSampleValues)
	}

	return profile, nil
}

// findColumn looks up column's catalog entry within ds, if documented.
func findColumn(ds *catalog.Dataset, column string) (catalog.Column, bool) {
	for _, c := range ds.Columns {
		if c.Name == column {
			return c, true
		}
	}
	return catalog.Column{}, false
}

// sortedValueCounts turns a value->count map into a slice sorted by count
// descending, breaking ties alphabetically for stable output.
func sortedValueCounts(counts map[string]int64) []ValueCount {
	vc := make([]ValueCount, 0, len(counts))
	for v, n := range counts {
		vc = append(vc, ValueCount{Value: v, Count: n})
	}
	sort.Slice(vc, func(i, j int) bool {
		if vc[i].Count != vc[j].Count {
			return vc[i].Count > vc[j].Count
		}
		return vc[i].Value < vc[j].Value
	})
	return vc
}

// sampleKeys returns up to n keys from counts, in descending-frequency
// order, as a lightweight preview for high-cardinality columns.
func sampleKeys(counts map[string]int64, n int) []string {
	vc := sortedValueCounts(counts)
	if len(vc) > n {
		vc = vc[:n]
	}
	samples := make([]string, len(vc))
	for i, v := range vc {
		samples[i] = v.Value
	}
	return samples
}

// toFloat converts a data.Cell's Go-native value to float64, if it's
// numeric.
func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	default:
		return 0, false
	}
}
