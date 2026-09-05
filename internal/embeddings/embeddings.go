// Package embeddings turns catalog metadata and user queries into vectors
// for search_catalog. Only metadata (dataset/column names, descriptions,
// units) and the user's query text are ever sent here — never row-level
// data from the parquet files.
package embeddings

import (
	"context"
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/config"
)

// Client embeds text for semantic search. Implementations are adapters for
// a specific provider's wire format.
//
// EmbedQuery and EmbedDocument are separate because some providers (Gemini)
// produce measurably better retrieval when a query and the documents it's
// matched against are embedded with different task hints — an asymmetric
// mode built for exactly search_catalog's shape (one query, many
// documents). Providers without that concept (Ollama) just treat both the
// same.
type Client interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedDocument(ctx context.Context, text string) ([]float32, error)
}

// NewClient builds a Client for whichever provider cfg.EmbeddingsProvider
// names ("ollama", the default, or "gemini").
func NewClient(cfg config.Config) (Client, error) {
	switch cfg.EmbeddingsProvider {
	case "", "ollama":
		return newOllamaClient(cfg), nil
	case "gemini":
		return newGeminiClient(cfg)
	default:
		return nil, fmt.Errorf("embeddings: unknown provider %q (want \"ollama\" or \"gemini\")", cfg.EmbeddingsProvider)
	}
}
