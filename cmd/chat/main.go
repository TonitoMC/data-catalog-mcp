// Command chat is a barebones agentic REPL: it connects to every MCP
// server listed in a config (same mcpServers schema as client.json),
// exposes their tools to a chat model, and executes whatever tool calls
// the model makes. The model is reached through internal/llm's
// OpenAI-compatible client — LLM_API_URL just needs to point at an
// OpenAI-compatible endpoint, whether that's Ollama's /v1, OpenAI itself,
// or any other server speaking the same wire format.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"

	"github.com/tonitomc/data-catalog-mcp/internal/client"
	"github.com/tonitomc/data-catalog-mcp/internal/llm"
)

const systemPrompt = "You are a helpful assistant with access to tools for a data catalog, " +
	"the local filesystem, and git. Use them when they help answer the user's question.\n\n" +
	"For open-ended or exploratory questions about what data is available (e.g. \"do we have " +
	"data on X\", \"what datasets cover Y\"), start with data-catalog__search_catalog rather " +
	"than listing every dataset — it ranks datasets/columns by relevance to a query. Reach for " +
	"data-catalog__list_datasets only when the user wants a full inventory, and " +
	"data-catalog__describe_dataset only once you already know which specific dataset you need."

// toolSep separates a tool's owning server from its own name in the
// qualified name exposed to the model (e.g. "git__git_status"). Chosen
// because MCP tool names use single underscores, so "__" stays
// unambiguous to split on.
const toolSep = "__"

func main() {
	_ = godotenv.Load()

	configPath := getEnv("CLIENT_CONFIG", "client.json")
	llmURL := getEnv("LLM_API_URL", "http://localhost:11434/v1")
	llmModel := getEnv("LLM_MODEL", "qwen2.5:7b-instruct")
	model := llm.New(llmURL, llmModel, os.Getenv("LLM_API_KEY"))

	cfg, err := client.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("chat: %v", err)
	}
	router, err := client.NewRouter(cfg)
	if err != nil {
		log.Fatalf("chat: %v", err)
	}
	defer router.Close()

	tools, resolver, err := collectTools(router)
	if err != nil {
		log.Fatalf("chat: %v", err)
	}

	header := fmt.Sprintf("%d tools across %v · %s (%s) · /exit to quit",
		len(tools), router.Servers(), llmModel, llmURL)

	tm := newChatModel(model, router, tools, resolver, header)
	p := tea.NewProgram(tm, tea.WithAltScreen())
	tm.program = p

	if _, err := p.Run(); err != nil {
		log.Fatalf("chat: %v", err)
	}
}

func runToolCall(tc llm.ToolCall, resolver *toolResolver, router *client.Router) string {
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

// toolResolver maps a qualified tool name (e.g. "git__git_status") back to
// its owning server and bare tool name. Small local models frequently
// mangle the exact separator/casing of a tool name they were just given
// (dots or hyphens instead of "__", wrong case, dropped server prefix) —
// resolveLoose tolerates that by comparing names with separators and case
// stripped, rather than failing outright and burning a turn on a
// hallucinated retry.
type toolResolver struct {
	owners     map[string]string // qualified name -> server
	normalized map[string]string // normalize(qualified name) -> qualified name
}

func newToolResolver() *toolResolver {
	return &toolResolver{owners: map[string]string{}, normalized: map[string]string{}}
}

func (r *toolResolver) add(server, toolName string) string {
	qualified := server + toolSep + toolName
	r.owners[qualified] = server
	r.normalized[normalizeToolName(qualified)] = qualified
	return qualified
}

// Resolve returns the owning server and bare tool name for a (possibly
// mangled) qualified name the model produced.
func (r *toolResolver) Resolve(name string) (server, toolName string, ok bool) {
	qualified := name
	server, ok = r.owners[qualified]
	if !ok {
		qualified, ok = r.normalized[normalizeToolName(name)]
		if !ok {
			return "", "", false
		}
		server = r.owners[qualified]
	}
	return server, strings.TrimPrefix(qualified, server+toolSep), true
}

// Names lists every known qualified tool name, for error messages that
// help the model self-correct.
func (r *toolResolver) Names() []string {
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

// collectTools gathers every server's tools into the model's tool schema,
// qualifying names with their owning server to avoid collisions, plus a
// resolver that maps a (possibly mangled) qualified name back to its
// owning server.
func collectTools(router *client.Router) ([]llm.Tool, *toolResolver, error) {
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

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
