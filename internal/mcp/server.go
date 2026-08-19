// Package mcp wires up the MCP server and its registered tools.
package mcp

import (
	"log"

	"github.com/tonitomc/data-catalog-mcp/internal/config"
)

// Server is the local MCP server. Tool registration will be added here
// as each tool (list_datasets, describe_dataset, profile_column, ...)
// is implemented.
type Server struct {
	cfg config.Config
}

// New builds a Server from the given config. Catalog loading, the parquet
// registry, and the embeddings index will be wired in here as they're built.
func New(cfg config.Config) *Server {
	return &Server{cfg: cfg}
}

// Run starts the server. For now this just logs that it would start,
// as a placeholder until the actual MCP transport (stdio) is wired up.
func (s *Server) Run() error {
	log.Printf("data-catalog-mcp: catalog=%s data=%s", s.cfg.CatalogPath, s.cfg.DataDir)
	log.Println("data-catalog-mcp: no transport wired up yet")
	return nil
}
