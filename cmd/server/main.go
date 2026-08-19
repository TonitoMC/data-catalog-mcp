// Command server starts the data-catalog-mcp server, speaking MCP over
// stdio (stdin/stdout carry the protocol; logs go to stderr so they don't
// corrupt the JSON-RPC stream).
package main

import (
	"log"
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

	conn := jsonrpc.NewConn(os.Stdin, os.Stdout)
	if err := srv.Run(conn); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
