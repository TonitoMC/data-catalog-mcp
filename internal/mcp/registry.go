package mcp

import (
	"errors"
	"log"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/config"
	"github.com/tonitomc/data-catalog-mcp/internal/embeddings"
)

// ErrInvalidArgument is returned by a Tool's Call when the caller-supplied
// arguments are missing or malformed, so the dispatcher can map it to a
// JSON-RPC "invalid params" error rather than an internal one.
var ErrInvalidArgument = errors.New("mcp: invalid argument")

// newRegistry builds the map of every tool this server exposes, keyed by
// name. Adding a tool means adding one entry here (plus its own
// tool_*.go file implementing Tool).
func newRegistry(cfg config.Config, cat *catalog.Catalog) map[string]Tool {
	var embed embeddings.Client
	if cfg.EmbeddingsProvider == "gemini" || (cfg.EmbeddingsAPIURL != "" && cfg.EmbeddingsModel != "") {
		var err error
		embed, err = embeddings.NewClient(cfg)
		if err != nil {
			log.Printf("mcp: embeddings unavailable, search_catalog will fall back to keyword matching: %v", err)
			embed = nil
		}
	}

	reg := []Tool{
		pingTool{},
		listDatasetsTool{cat: cat, dataDir: cfg.DataDir},
		describeDatasetTool{cat: cat, dataDir: cfg.DataDir},
		profileColumnTool{cat: cat, dataDir: cfg.DataDir},
		validateDatasetTool{cat: cat, dataDir: cfg.DataDir},
		findUndocumentedColumnsTool{cat: cat, dataDir: cfg.DataDir},
		searchCatalogTool{cat: cat, embed: embed},
	}

	byName := make(map[string]Tool, len(reg))
	for _, t := range reg {
		byName[t.Name()] = t
	}
	return byName
}
