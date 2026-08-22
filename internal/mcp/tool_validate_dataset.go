package mcp

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// validateDatasetTool checks a dataset's actual data against its catalog
// quality rules.
type validateDatasetTool struct {
	cat     *catalog.Catalog
	dataDir string
}

func (validateDatasetTool) Name() string { return "validate_dataset" }
func (validateDatasetTool) Description() string {
	return "Validates a dataset's data against its catalog quality rules (not_null, unique, range, allowed_values, regex, type_check, row_count) and reports any violations."
}
func (validateDatasetTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dataset": map[string]any{
				"type":        "string",
				"description": "Dataset name, as returned by list_datasets.",
			},
		},
		"required": []string{"dataset"},
	}
}
func (t validateDatasetTool) Call(args map[string]any) (any, error) {
	dataset, _ := args["dataset"].(string)
	if dataset == "" {
		return nil, fmt.Errorf("%w: missing required argument: dataset", ErrInvalidArgument)
	}
	return tools.ValidateDataset(t.cat, t.dataDir, dataset)
}
