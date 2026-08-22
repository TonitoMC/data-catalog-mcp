package mcp

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// describeDatasetTool describes one registered dataset in full.
type describeDatasetTool struct {
	cat     *catalog.Catalog
	dataDir string
}

func (describeDatasetTool) Name() string { return "describe_dataset" }
func (describeDatasetTool) Description() string {
	return "Describes one registered dataset: its metadata, row count, and column dictionary."
}
func (describeDatasetTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Dataset name, as returned by list_datasets.",
			},
		},
		"required": []string{"name"},
	}
}
func (t describeDatasetTool) Call(args map[string]any) (any, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("%w: missing required argument: name", ErrInvalidArgument)
	}
	return tools.DescribeDataset(t.cat, t.dataDir, name)
}
