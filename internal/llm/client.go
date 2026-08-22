package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// client talks to any OpenAI-compatible /v1/chat/completions endpoint.
// That covers OpenAI itself and every local runner that mimics its wire
// format — Ollama (at http://localhost:11434/v1), vLLM, llama.cpp
// server, etc — so one implementation is all this package needs. Tool
// call arguments are a JSON string on the wire (not an object), and tool
// result messages must reference the call they're answering by ID; this
// type's only job is translating that wire format to/from the generic
// llm types.
type client struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client for an OpenAI-compatible API at baseURL (e.g.
// https://api.openai.com/v1, or http://localhost:11434/v1 for Ollama),
// using the given model for every call. apiKey may be empty for servers
// that don't require auth.
func New(baseURL, model, apiKey string) Client {
	return &client{
		baseURL:    baseURL,
		model:      model,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// wireMessage/wireToolCall mirror the OpenAI wire format, where tool call
// arguments are a JSON-encoded string rather than an object.
type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type"`
	Function wireToolCallFunc `json:"function"`
}

type wireToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionsRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message wireMessage `json:"message"`
	} `json:"choices"`
}

func (c *client) Chat(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	wireMessages := make([]wireMessage, len(messages))
	for i, m := range messages {
		wm, err := toWireMessage(m)
		if err != nil {
			return Message{}, fmt.Errorf("llm: %w", err)
		}
		wireMessages[i] = wm
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model:    c.model,
		Messages: wireMessages,
		Tools:    tools,
	})
	if err != nil {
		return Message{}, fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("llm: unexpected status %d: %s", resp.StatusCode, b)
	}

	var out chatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Message{}, fmt.Errorf("llm: decode response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Message{}, fmt.Errorf("llm: response had no choices")
	}
	return fromWireMessage(out.Choices[0].Message)
}

func toWireMessage(m Message) (wireMessage, error) {
	wm := wireMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
	for _, tc := range m.ToolCalls {
		args, err := json.Marshal(tc.Function.Arguments)
		if err != nil {
			return wireMessage{}, fmt.Errorf("encode tool call arguments: %w", err)
		}
		wm.ToolCalls = append(wm.ToolCalls, wireToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: wireToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: string(args),
			},
		})
	}
	return wm, nil
}

func fromWireMessage(wm wireMessage) (Message, error) {
	m := Message{Role: wm.Role, Content: wm.Content, ToolCallID: wm.ToolCallID}
	for _, wtc := range wm.ToolCalls {
		var args map[string]any
		if wtc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(wtc.Function.Arguments), &args); err != nil {
				return Message{}, fmt.Errorf("decode tool call arguments: %w", err)
			}
		}
		m.ToolCalls = append(m.ToolCalls, ToolCall{
			ID: wtc.ID,
			Function: ToolCallFunction{
				Name:      wtc.Function.Name,
				Arguments: args,
			},
		})
	}
	return m, nil
}
