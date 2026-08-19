package catalog

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load parses the catalog.yaml file at path.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog: read %s: %w", path, err)
	}

	var cat Catalog
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("catalog: parse %s: %w", path, err)
	}
	return &cat, nil
}

// UnmarshalYAML lets Rule accept either a bare scalar (e.g. `not_null`) or
// a mapping with one rule key plus an optional `severity`
// (e.g. `range: [0, 600]` / `severity: error`).
func (r *Rule) UnmarshalYAML(value *yaml.Node) error {
	r.Severity = "error"

	if value.Kind == yaml.ScalarNode {
		r.Kind = value.Value
		return nil
	}

	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("catalog: rule must be a string or mapping, got kind %d", value.Kind)
	}

	r.Args = map[string]any{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		valNode := value.Content[i+1]

		if key == "severity" {
			r.Severity = valNode.Value
			continue
		}

		r.Kind = key
		var decoded any
		if err := valNode.Decode(&decoded); err != nil {
			return fmt.Errorf("catalog: decode rule %q: %w", key, err)
		}
		r.Args["value"] = decoded
	}
	return nil
}
