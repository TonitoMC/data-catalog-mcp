// Package llm defines a provider-agnostic chat interface with tool
// calling, so cmd/api doesn't care whether it's talking to a local
// Ollama instance or an OpenAI-compatible API.
package llm

import "context"

// Message is one turn in a chat. An assistant message may carry
// ToolCalls instead of Content. A tool-result message sets Role "tool",
// Content to the result, and ToolCallID to the call it's answering
// (required by OpenAI-compatible APIs; ignored by Ollama).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is one function invocation the model requested. ID is set by
// OpenAI-compatible APIs and must be echoed back in the matching tool
// result message; Ollama doesn't use it.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Tool describes one callable function, in the OpenAI-style function
// calling shape both Ollama and OpenAI-compatible APIs share.
type Tool struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ToolCallStarted announces that a tool call's name has become known
// mid-stream (its arguments may still be arriving in later chunks).
type ToolCallStarted struct {
	Index int
	Name  string
}

// StreamDelta is one increment of a streamed response. Exactly one of
// Content, ReasoningContent, ToolCallStarted, or Done is meaningful per
// value; Err is set on failure, after which the stream ends.
type StreamDelta struct {
	Content string // incremental answer text, if any this chunk

	// ReasoningContent carries the model's reasoning/thinking text, on
	// providers that stream it separately from the final answer (DeepSeek
	// and compatible APIs use "reasoning_content" on the wire). Always
	// empty on providers that don't support it — there's no reasoning to
	// separate out, not a parsing failure.
	ReasoningContent string

	ToolCallStarted *ToolCallStarted // set once, the first time a given tool call's name is known
	Done            bool             // true on the final value; Message is populated
	Message         Message          // the fully assembled message; valid only when Done
	Err             error
}

// Client is a chat model with tool calling. Implementations are adapters
// for a specific provider's wire format; the provider and model are fixed
// at construction time.
type Client interface {
	// Chat sends the full message history (plus available tools) and
	// returns the model's next message. Callers are expected to resend
	// the whole history each call, since providers are stateless between
	// requests.
	Chat(ctx context.Context, messages []Message, tools []Tool) (Message, error)

	// ChatStream is like Chat but streams the response incrementally over
	// the returned channel, which is closed once the response is fully
	// read (successfully or not — check the last value's Err).
	ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamDelta, error)
}
