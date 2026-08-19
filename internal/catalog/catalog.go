// Package catalog holds the parsed data dictionary + quality rules
// declared in catalog.yaml — the human-authored half of the two data
// sources this server combines (the other being the Parquet files
// themselves, see internal/data).
package catalog

// Catalog is the full parsed catalog.yaml.
type Catalog struct {
	Datasets []Dataset `yaml:"datasets"`
}

// Dataset describes one registered dataset and its columns.
type Dataset struct {
	Name         string   `yaml:"name"`
	File         string   `yaml:"file"`
	Description  string   `yaml:"description"`
	Origin       string   `yaml:"origin"`
	Columns      []Column `yaml:"columns"`
	DatasetRules []Rule   `yaml:"dataset_rules"`
}

// Column describes one column's type, meaning, and quality rules.
type Column struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Unit        string `yaml:"unit"`
	Rules       []Rule `yaml:"rules"`
}

// Rule is one quality rule, e.g. `not_null`, `range: [0, 600]`, or
// `row_count: {min: 1}`. Kind is the rule name (not_null, unique, range,
// allowed_values, regex, type_check, row_count, freshness); Args holds
// whatever value followed it in the YAML, if any. Severity defaults to
// "error" when not declared.
type Rule struct {
	Kind     string
	Severity string
	Args     map[string]any
}

// Dataset looks up a dataset by name.
func (c *Catalog) Dataset(name string) (*Dataset, bool) {
	for i := range c.Datasets {
		if c.Datasets[i].Name == name {
			return &c.Datasets[i], true
		}
	}
	return nil, false
}
