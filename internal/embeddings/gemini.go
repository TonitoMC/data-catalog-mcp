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

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// geminiClient embeds text via the Gemini API's embedContent endpoint.
// Unlike Ollama, Gemini supports an explicit task hint per call —
// RETRIEVAL_QUERY for the search query, RETRIEVAL_DOCUMENT for the
// documents being searched — an asymmetric mode built for exactly
// search_catalog's shape. gemini-embedding-2 dropped taskType in favor of
// prompt-prefix instructions; this client targets gemini-embedding-001
// (or whatever EMBEDDINGS_MODEL names) specifically because it still
// supports taskType directly.
type geminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func newGeminiClient(cfg config.Config) (*geminiClient, error) {
	if cfg.EmbeddingsAPIKey == "" {
		return nil, fmt.Errorf("embeddings: EMBEDDINGS_API_KEY is required for the gemini provider")
	}
	model := cfg.EmbeddingsModel
	if model == "" {
		model = "gemini-embedding-001"
	}
	return &geminiClient{
		apiKey:     cfg.EmbeddingsAPIKey,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

type geminiEmbedRequest struct {
	TaskType string             `json:"taskType,omitempty"`
	Content  geminiEmbedContent `json:"content"`
}

type geminiEmbedContent struct {
	Parts []geminiEmbedPart `json:"parts"`
}

type geminiEmbedPart struct {
	Text string `json:"text"`
}

// geminiEmbedResponse is embedContent's response shape per Gemini API
// docs: a single "embedding" object holding the vector under "values".
// (batchEmbedContents returns a plural "embeddings" array instead — not
// used here since we embed one text per call.)
type geminiEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

func (c *geminiClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, "RETRIEVAL_QUERY")
}

func (c *geminiClient) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, "RETRIEVAL_DOCUMENT")
}

func (c *geminiClient) embed(ctx context.Context, text, taskType string) ([]float32, error) {
	body, err := json.Marshal(geminiEmbedRequest{
		TaskType: taskType,
		Content:  geminiEmbedContent{Parts: []geminiEmbedPart{{Text: text}}},
	})
	if err != nil {
		return nil, fmt.Errorf("embeddings: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:embedContent", geminiBaseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embeddings: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embeddings: unexpected status %d: %s", resp.StatusCode, b)
	}

	var out geminiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	return out.Embedding.Values, nil
}
