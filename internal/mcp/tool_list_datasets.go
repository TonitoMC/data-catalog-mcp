package mcp

import (
	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// listDatasetsTool lists every dataset registered in the catalog.
type listDatasetsTool struct {
	cat     *catalog.Catalog
	dataDir string
}

func (listDatasetsTool) Name() string { return "list_datasets" }
func (listDatasetsTool) Description() string {
	return "Lists the registered datasets with their description, origin, and row count."
}
func (listDatasetsTool) InputSchema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}
func (t listDatasetsTool) Call(args map[string]any) (any, error) {
	return tools.ListDatasets(t.cat, t.dataDir)
}
