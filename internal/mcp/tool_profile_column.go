package mcp

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// profileColumnTool summarizes the actual values in one dataset column.
type profileColumnTool struct {
	cat     *catalog.Catalog
	dataDir string
}

func (profileColumnTool) Name() string { return "profile_column" }
func (profileColumnTool) Description() string {
	return "Profiles one column of a dataset: declared type, count, null count/ratio, distinct count, numeric min/max/mean, and either a full value-frequency table (low-cardinality columns) or sample values (high-cardinality columns)."
}
func (profileColumnTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dataset": map[string]any{
				"type":        "string",
				"description": "Dataset name, as returned by list_datasets.",
			},
			"column": map[string]any{
				"type":        "string",
				"description": "Column name, as returned by describe_dataset.",
			},
		},
		"required": []string{"dataset", "column"},
	}
}
func (t profileColumnTool) Call(args map[string]any) (any, error) {
	dataset, _ := args["dataset"].(string)
	column, _ := args["column"].(string)
	if dataset == "" || column == "" {
		return nil, fmt.Errorf("%w: missing required argument(s): dataset, column", ErrInvalidArgument)
	}
	return tools.ProfileColumn(t.cat, t.dataDir, dataset, column)
}
