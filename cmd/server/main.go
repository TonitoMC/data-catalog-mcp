// Command server starts the data-catalog-mcp server. By default it speaks
// MCP over stdio (stdin/stdout carry the protocol; logs go to stderr so
// they don't corrupt the JSON-RPC stream) — the shape an MCP host spawns
// as a local subprocess. Set TRANSPORT=http to instead serve MCP over
// plain HTTP on PORT (or HTTP_ADDR), the shape a host reaches over the
// network — e.g. this same binary running on Cloud Run.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tonitomc/data-catalog-mcp/internal/config"
	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
	"github.com/tonitomc/data-catalog-mcp/internal/mcp"
)

func main() {
	cfg := config.Load()

	srv, err := mcp.New(cfg)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	if getEnv("TRANSPORT", "stdio") == "http" {
		runHTTP(srv)
		return
	}

	conn := jsonrpc.NewConn(os.Stdin, os.Stdout)
	if err := srv.Run(conn); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}

func runHTTP(srv *mcp.Server) {
	// PORT is Cloud Run's convention (it sets this itself); HTTP_ADDR is
	// the local-dev override so you're not fighting Cloud Run's env var
	// during manual testing.
	addr := getEnv("HTTP_ADDR", ":"+getEnv("PORT", "8091"))
	log.Printf("server: listening on %s (MCP over HTTP)", addr)
	if err := http.ListenAndServe(addr, srv.HTTPHandler()); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
