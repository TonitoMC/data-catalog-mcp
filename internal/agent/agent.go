// Package agent holds the tool-call plumbing for cmd/api: qualifying tool
// names by their owning server, resolving a (possibly mangled) qualified
// name back to a server + tool call, and streaming one turn of the
// tool-call loop against an llm.Client and a client.Router.
package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tonitomc/data-catalog-mcp/internal/client"
	"github.com/tonitomc/data-catalog-mcp/internal/llm"
)

// SystemPrompt is the default instruction set for a host that exposes the
// data-catalog, filesystem, and git servers together.
const SystemPrompt = "You are a helpful assistant with access to tools for a data catalog, " +
	"the local filesystem, and git. Use them when they help answer the user's question.\n\n" +
	"For open-ended or exploratory questions about what data is available (e.g. \"do we have " +
	"data on X\", \"what datasets cover Y\"), start with data-catalog__search_catalog rather " +
	"than listing every dataset — it ranks datasets/columns by relevance to a query. Reach for " +
	"data-catalog__list_datasets only when the user wants a full inventory, and " +
	"data-catalog__describe_dataset only once you already know which specific dataset you need.\n\n" +
	"When your answer includes a markdown table, always leave a blank line before it, and make " +
	"sure the header row, the separator row, and every data row have exactly the same number of " +
	"columns — a mismatched or unspaced table renders as broken text instead of a table."

// ToolSep separates a tool's owning server from its own name in the
// qualified name exposed to the model (e.g. "git__git_status"). Chosen
// because MCP tool names use single underscores, so "__" stays
// unambiguous to split on.
const ToolSep = "__"

// ToolResolver maps a qualified tool name (e.g. "git__git_status") back to
// its owning server and bare tool name. Small local models frequently
// mangle the exact separator/casing of a tool name they were just given
// (dots or hyphens instead of "__", wrong case, dropped server prefix) —
// Resolve tolerates that by comparing names with separators and case
// stripped, rather than failing outright and burning a turn on a
// hallucinated retry.
type ToolResolver struct {
	owners     map[string]string // qualified name -> server
	normalized map[string]string // normalize(qualified name) -> qualified name
}

func newToolResolver() *ToolResolver {
	return &ToolResolver{owners: map[string]string{}, normalized: map[string]string{}}
}

func (r *ToolResolver) add(server, toolName string) string {
	qualified := server + ToolSep + toolName
	r.owners[qualified] = server
	r.normalized[normalizeToolName(qualified)] = qualified
	return qualified
}

// Resolve returns the owning server and bare tool name for a (possibly
// mangled) qualified name the model produced.
func (r *ToolResolver) Resolve(name string) (server, toolName string, ok bool) {
	qualified := name
	server, ok = r.owners[qualified]
	if !ok {
		qualified, ok = r.normalized[normalizeToolName(name)]
		if !ok {
			return "", "", false
		}
		server = r.owners[qualified]
	}
	return server, strings.TrimPrefix(qualified, server+ToolSep), true
}

// Names lists every known qualified tool name, for error messages that
// help the model self-correct.
func (r *ToolResolver) Names() []string {
	names := make([]string, 0, len(r.owners))
	for name := range r.owners {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// normalizeToolName strips separators and case so "data_catalog.describe_dataset"
// and "data-catalog__describe_dataset" compare equal.
func normalizeToolName(name string) string {
	name = strings.ToLower(name)
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == '.' || r == ' ' {
			return -1
		}
		return r
	}, name)
}

// CollectTools gathers every configured server's tools into the model's
// tool schema, qualifying names with their owning server to avoid
// collisions, plus a resolver that maps a (possibly mangled) qualified
// name back to its owning server.
func CollectTools(router *client.Router) ([]llm.Tool, *ToolResolver, error) {
	var tools []llm.Tool
	resolver := newToolResolver()

	for _, server := range router.Servers() {
		serverTools, err := router.ListTools(server)
		if err != nil {
			return nil, nil, fmt.Errorf("list tools for %q: %w", server, err)
		}
		for _, t := range serverTools {
			qualified := resolver.add(server, t.Name)
			tools = append(tools, llm.Tool{
				Type: "function",
				Function: llm.ToolFunction{
					Name:        qualified,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}
	return tools, resolver, nil
}

// RunToolCall resolves tc's qualified name to a server + tool and invokes
// it through router, returning the result text (or an error string if the
// name doesn't resolve or the call itself fails).
func RunToolCall(tc llm.ToolCall, resolver *ToolResolver, router *client.Router) string {
	server, toolName, ok := resolver.Resolve(tc.Function.Name)
	if !ok {
		return fmt.Sprintf("error: unknown tool %q (available: %s)", tc.Function.Name, strings.Join(resolver.Names(), ", "))
	}

	result, err := router.CallTool(server, toolName, tc.Function.Arguments)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

// Event is one increment of a streamed turn, emitted through StreamTurn's
// callback. Exactly one of Content/ToolName+friends/Answer+History/Error
// is meaningful per event, per its Type.
type Event struct {
	Type string `json:"type"` // "delta" | "reasoning_delta" | "tool_call_start" | "tool_call_result" | "done" | "error"

	// Content carries the incremental text for both "delta" (the answer)
	// and "reasoning_delta" (the model's reasoning, on providers that
	// stream it separately) — the Type is what tells a reader which
	// channel this piece belongs to.
	Content string `json:"content,omitempty"`

	ToolName   string         `json:"toolName,omitempty"`   // "tool_call_start" | "tool_call_result"
	ToolArgs   map[string]any `json:"toolArgs,omitempty"`   // "tool_call_result"
	ToolResult string         `json:"toolResult,omitempty"` // "tool_call_result"

	Answer  string        `json:"answer,omitempty"`  // "done": the turn's final answer text
	History []llm.Message `json:"history,omitempty"` // "done": full history, to resend next turn

	Error string `json:"error,omitempty"` // "error"
}

// StreamTurn drives the tool-call loop for one user turn, streaming
// progress through emit as it happens: answer text as the model produces
// it, each tool call as soon as its name is known, each tool call's
// result once it's run, and finally the assembled answer plus updated
// history. Emits a single "error" event and stops on any failure, so the
// caller (an HTTP handler forwarding these as server-sent events) never
// needs its own error path — everything the client needs is already an
// event.
func StreamTurn(ctx context.Context, llmClient llm.Client, router *client.Router, tools []llm.Tool, resolver *ToolResolver, history []llm.Message, emit func(Event)) {
	history = append([]llm.Message(nil), history...)

	for {
		// A turn can take several rounds (stream some text, call a tool,
		// stream more text after the result). Every round but the last one
		// produces throwaway lead-in text ("Let me check that...") ahead of
		// its tool calls — real model output, but not part of the final
		// answer once it's overwritten by the next round. Marking the
		// boundary lets the frontend file that leftover text under
		// reasoning instead of letting it bleed into (and then vanish from)
		// the visible answer.
		emit(Event{Type: "round_start"})

		stream, err := llmClient.ChatStream(ctx, history, tools)
		if err != nil {
			emit(Event{Type: "error", Error: fmt.Sprintf("talking to model: %v", err)})
			return
		}

		var reply llm.Message
		for d := range stream {
			switch {
			case d.Err != nil:
				emit(Event{Type: "error", Error: d.Err.Error()})
				return
			case d.Content != "":
				emit(Event{Type: "delta", Content: d.Content})
			case d.ReasoningContent != "":
				emit(Event{Type: "reasoning_delta", Content: d.ReasoningContent})
			case d.ToolCallStarted != nil:
				emit(Event{Type: "tool_call_start", ToolName: d.ToolCallStarted.Name})
			case d.Done:
				reply = d.Message
			}
		}
		history = append(history, reply)

		if len(reply.ToolCalls) == 0 {
			emit(Event{Type: "done", Answer: reply.Content, History: history})
			return
		}

		for _, tc := range reply.ToolCalls {
			result := RunToolCall(tc, resolver, router)
			emit(Event{
				Type:       "tool_call_result",
				ToolName:   tc.Function.Name,
				ToolArgs:   tc.Function.Arguments,
				ToolResult: result,
			})
			history = append(history, llm.Message{Role: "tool", Content: result, ToolCallID: tc.ID})
		}
	}
}
