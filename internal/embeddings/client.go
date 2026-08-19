// Package embeddings talks to a local Ollama instance to turn catalog
// metadata and user queries into vectors for search_catalog.
//
// Only metadata (dataset/column names, descriptions, units) and the user's
// query text are ever sent here — never row-level data from the parquet
// files.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tonitomc/data-catalog-mcp/internal/config"
)

// Client embeds text via Ollama's /api/embeddings endpoint.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient builds a Client from config. BaseURL/model default to a local
// Ollama instance running nomic-embed-text (see config.Load).
func NewClient(cfg config.Config) *Client {
	return &Client{
		baseURL: cfg.EmbeddingsAPIURL,
		model:   cfg.EmbeddingsModel,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed returns the embedding vector for a single piece of text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("embeddings: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embeddings: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embeddings: unexpected status %d: %s", resp.StatusCode, b)
	}

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	return out.Embedding, nil
}
