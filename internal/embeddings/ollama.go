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

// ollamaClient embeds text via Ollama's /api/embeddings endpoint. Ollama
// has no query-vs-document distinction, so EmbedQuery and EmbedDocument
// both just call the same endpoint.
type ollamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func newOllamaClient(cfg config.Config) *ollamaClient {
	return &ollamaClient{
		baseURL:    cfg.EmbeddingsAPIURL,
		model:      cfg.EmbeddingsModel,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (c *ollamaClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text)
}

func (c *ollamaClient) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text)
}

func (c *ollamaClient) embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaEmbedRequest{Model: c.model, Prompt: text})
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

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	return out.Embedding, nil
}
