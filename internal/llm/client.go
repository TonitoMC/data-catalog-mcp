package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
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
	Stream   bool          `json:"stream,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message wireMessage `json:"message"`
	} `json:"choices"`
}

func (c *client) Chat(ctx context.Context, messages []Message, tools []Tool) (Message, error) {
	wireMessages, err := toWireMessages(messages)
	if err != nil {
		return Message{}, err
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model:    c.model,
		Messages: wireMessages,
		Tools:    tools,
	})
	if err != nil {
		return Message{}, fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := c.newRequest(ctx, body)
	if err != nil {
		return Message{}, err
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

// streamChunk is one server-sent chunk of a streamed chat completion.
// ReasoningContent is a de-facto extension (DeepSeek and compatible
// providers, including OpenCode Go's deepseek-* models) carrying the
// model's reasoning/thinking text on its own channel, separate from the
// final answer in Content — plain OpenAI-compatible servers simply never
// set it.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string              `json:"content"`
			ReasoningContent string              `json:"reasoning_content"`
			ToolCalls        []wireToolCallDelta `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// wireToolCallDelta is one incremental slice of a tool call as it streams
// in: Index identifies which tool call (of possibly several in the same
// response) this piece belongs to; ID/Name arrive once, early; Arguments
// arrives in pieces that must be concatenated in order.
type wireToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

func (c *client) ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamDelta, error) {
	wireMessages, err := toWireMessages(messages)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(chatCompletionsRequest{
		Model:    c.model,
		Messages: wireMessages,
		Tools:    tools,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := c.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("llm: unexpected status %d: %s", resp.StatusCode, b)
	}

	ch := make(chan StreamDelta)
	go streamChatCompletions(resp.Body, ch)
	return ch, nil
}

// accumulatingToolCall gathers one tool call's fields as they arrive
// across multiple stream chunks, keyed by the wire delta's Index.
type accumulatingToolCall struct {
	id   string
	name string
	args strings.Builder
}

// streamChatCompletions reads an OpenAI-compatible SSE response body
// ("data: {...}" lines, terminated by "data: [DONE]"), reassembles
// incremental content and tool-call deltas, and sends them (plus a final
// Done value carrying the fully assembled Message) on ch. Always closes
// body and ch before returning.
func streamChatCompletions(body io.ReadCloser, ch chan<- StreamDelta) {
	defer close(ch)
	defer body.Close()

	var content strings.Builder
	calls := map[int]*accumulatingToolCall{}
	announced := map[int]bool{}
	order := []int{}

	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" && strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				break
			}

			var chunk streamChunk
			if jsonErr := json.Unmarshal([]byte(payload), &chunk); jsonErr != nil {
				ch <- StreamDelta{Err: fmt.Errorf("llm: decode stream chunk: %w", jsonErr)}
				return
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.ReasoningContent != "" {
					ch <- StreamDelta{ReasoningContent: delta.ReasoningContent}
				}
				if delta.Content != "" {
					content.WriteString(delta.Content)
					ch <- StreamDelta{Content: delta.Content}
				}
				for _, tc := range delta.ToolCalls {
					a, ok := calls[tc.Index]
					if !ok {
						a = &accumulatingToolCall{}
						calls[tc.Index] = a
						order = append(order, tc.Index)
					}
					if tc.ID != "" {
						a.id = tc.ID
					}
					if tc.Function.Name != "" {
						a.name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						a.args.WriteString(tc.Function.Arguments)
					}
					if a.name != "" && !announced[tc.Index] {
						announced[tc.Index] = true
						ch <- StreamDelta{ToolCallStarted: &ToolCallStarted{Index: tc.Index, Name: a.name}}
					}
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				ch <- StreamDelta{Err: fmt.Errorf("llm: read stream: %w", err)}
				return
			}
			break
		}
	}

	toolCalls := make([]ToolCall, 0, len(order))
	sort.Ints(order)
	for _, idx := range order {
		a := calls[idx]
		var args map[string]any
		if a.args.Len() > 0 {
			if err := json.Unmarshal([]byte(a.args.String()), &args); err != nil {
				ch <- StreamDelta{Err: fmt.Errorf("llm: decode tool call arguments: %w", err)}
				return
			}
		}
		toolCalls = append(toolCalls, ToolCall{ID: a.id, Function: ToolCallFunction{Name: a.name, Arguments: args}})
	}

	ch <- StreamDelta{Done: true, Message: Message{Role: "assistant", Content: content.String(), ToolCalls: toolCalls}}
}

func (c *client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

func toWireMessages(messages []Message) ([]wireMessage, error) {
	wireMessages := make([]wireMessage, len(messages))
	for i, m := range messages {
		wm, err := toWireMessage(m)
		if err != nil {
			return nil, fmt.Errorf("llm: %w", err)
		}
		wireMessages[i] = wm
	}
	return wireMessages, nil
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
