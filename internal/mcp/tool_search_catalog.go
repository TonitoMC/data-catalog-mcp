package mcp

import (
	"context"
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/embeddings"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// searchCatalogTool finds datasets/columns whose catalog metadata best
// matches a natural-language query. embed is nil when no embeddings
// service is configured, in which case tools.SearchCatalog falls back to
// keyword matching.
type searchCatalogTool struct {
	cat   *catalog.Catalog
	embed embeddings.Client
}

func (searchCatalogTool) Name() string { return "search_catalog" }
func (searchCatalogTool) Description() string {
	return "Searches dataset and column descriptions for the ones most relevant to a query. Uses semantic search when an embeddings service is configured, otherwise keyword matching."
}
func (searchCatalogTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Natural-language search query.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default 5).",
			},
		},
		"required": []string{"query"},
	}
}
func (t searchCatalogTool) Call(args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("%w: missing required argument: query", ErrInvalidArgument)
	}
	limit := 0
	if l, ok := args["limit"].(float64); ok { // JSON numbers decode as float64
		limit = int(l)
	}
	return tools.SearchCatalog(context.Background(), t.cat, t.embed, query, limit)
}
