package mcp

import (
	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// findUndocumentedColumnsTool flags Parquet columns that aren't declared
// in the catalog's data dictionary.
type findUndocumentedColumnsTool struct {
	cat     *catalog.Catalog
	dataDir string
}

func (findUndocumentedColumnsTool) Name() string { return "find_undocumented_columns" }
func (findUndocumentedColumnsTool) Description() string {
	return "Finds Parquet columns not declared in the catalog's data dictionary. Checks one dataset if given, otherwise every registered dataset."
}
func (findUndocumentedColumnsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dataset": map[string]any{
				"type":        "string",
				"description": "Dataset name to check. Omit to check every registered dataset.",
			},
		},
	}
}
func (t findUndocumentedColumnsTool) Call(args map[string]any) (any, error) {
	dataset, _ := args["dataset"].(string)
	return tools.FindUndocumentedColumns(t.cat, t.dataDir, dataset)
}
