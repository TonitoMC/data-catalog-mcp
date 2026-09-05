// Command api is the MCP host for the web frontend: it connects to every
// MCP server listed in a config (same mcpServers schema as client.json)
// once at startup, and serves what it found — which servers came up and
// what tools each one offers — as JSON. It also drives the tool-call loop
// (see internal/agent), streamed to the browser as server-sent events, so
// a page can hold a real conversation and watch the model's answer and
// every tool call arrive as they happen instead of waiting on one long
// blocking request.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/tonitomc/data-catalog-mcp/internal/agent"
	"github.com/tonitomc/data-catalog-mcp/internal/client"
	"github.com/tonitomc/data-catalog-mcp/internal/llm"
)

// serverInfo is one connected MCP server and the tools it reported at
// startup.
type serverInfo struct {
	Name  string            `json:"name"`
	Tools []client.ToolInfo `json:"tools"`
}

// serversResponse is what GET /api/servers returns: not just the
// connected MCP servers, but the actual chat backend this process was
// started with — so the frontend can show the real model/endpoint in use
// instead of a value baked into the UI at build time.
type serversResponse struct {
	LLMModel string       `json:"llmModel"`
	LLMURL   string       `json:"llmURL"`
	Servers  []serverInfo `json:"servers"`
}

// chatRequest carries the conversation so far (opaque — whatever the
// previous response's final "done" event's History was, or empty for a
// new conversation) plus the new user message.
type chatRequest struct {
	History []llm.Message `json:"history"`
	Message string        `json:"message"`
}

func main() {
	_ = godotenv.Load()

	configPath := getEnv("CLIENT_CONFIG", "client.json")
	addr := getEnv("API_ADDR", ":8090")
	llmURL := getEnv("LLM_API_URL", "http://localhost:11434/v1")
	llmModel := getEnv("LLM_MODEL", "qwen3:8b")
	llmClient := llm.New(llmURL, llmModel, os.Getenv("LLM_API_KEY"))

	cfg, err := client.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("api: %v", err)
	}
	router, err := client.NewRouter(cfg)
	if err != nil {
		log.Fatalf("api: %v", err)
	}
	defer router.Close()

	tools, resolver, err := agent.CollectTools(router)
	if err != nil {
		log.Fatalf("api: %v", err)
	}

	servers, err := collectServers(router)
	if err != nil {
		log.Fatalf("api: %v", err)
	}
	serversBody, err := json.Marshal(serversResponse{LLMModel: llmModel, LLMURL: llmURL, Servers: servers})
	if err != nil {
		log.Fatalf("api: encode servers: %v", err)
	}
	log.Printf("api: connected to %d server(s) %v, %d tools, model %s (%s)",
		len(servers), router.Servers(), len(tools), llmModel, llmURL)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(serversBody)
	})

	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		history := req.History
		if len(history) == 0 {
			history = []llm.Message{{Role: "system", Content: agent.SystemPrompt}}
		}
		history = append(history, llm.Message{Role: "user", Content: req.Message})

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		agent.StreamTurn(r.Context(), llmClient, router, tools, resolver, history, func(e agent.Event) {
			b, err := json.Marshal(e)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		})
	})

	log.Printf("api: listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("api: %v", err)
	}
}

// withCORS allows the Vite dev server (a different origin) to call this
// API, and answers the preflight OPTIONS request the browser sends ahead
// of any POST with a JSON body.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// collectServers snapshots every configured server's tool list once, in
// the router's stable (sorted) server order.
func collectServers(router *client.Router) ([]serverInfo, error) {
	names := router.Servers()
	servers := make([]serverInfo, 0, len(names))
	for _, name := range names {
		tools, err := router.ListTools(name)
		if err != nil {
			return nil, err
		}
		servers = append(servers, serverInfo{Name: name, Tools: tools})
	}
	return servers, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
