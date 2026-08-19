// Command server starts the data-catalog-mcp server.
package main

import (
	"log"

	"github.com/tonitomc/data-catalog-mcp/internal/config"
	"github.com/tonitomc/data-catalog-mcp/internal/mcp"
)

func main() {
	// Load config
	cfg := config.Load()

	// Start server
	srv := mcp.New(cfg)
	if err := srv.Run(); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
