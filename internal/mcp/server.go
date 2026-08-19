// Package mcp implements just enough of the Model Context Protocol
// (JSON-RPC 2.0 over stdio, per internal/jsonrpc) to boot a server, shake
// hands with a host, and serve tool calls.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/config"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
)

// Server dispatches JSON-RPC requests to MCP method handlers.
type Server struct {
	cfg config.Config
	cat *catalog.Catalog
}

// New builds a Server from the given config, loading the catalog up front
// so a bad catalog.yaml fails fast at startup rather than on first call.
func New(cfg config.Config) (*Server, error) {
	cat, err := catalog.Load(cfg.CatalogPath)
	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
	return &Server{cfg: cfg, cat: cat}, nil
}

// Run reads requests from conn until the input stream closes, dispatching
// each to the matching handler and writing back a response (unless the
// request was a notification, which expects none).
func (s *Server) Run(conn *jsonrpc.Conn) error {
	for {
		req, err := conn.ReadRequest()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		resp := s.handle(req)
		if req.IsNotification() {
			continue // no reply expected, even on error
		}
		if err := conn.WriteResponse(resp); err != nil {
			return err
		}
	}
}

func (s *Server) handle(req *jsonrpc.Request) *jsonrpc.Response {
	switch req.Method {
	case "initialize":
		return jsonrpc.NewResult(req.ID, initializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities:    capability{Tools: map[string]any{}},
			ServerInfo:      serverInfo{Name: "data-catalog-mcp", Version: "0.0.1"},
		})

	case "notifications/initialized":
		return nil // no response for notifications

	case "tools/list":
		return jsonrpc.NewResult(req.ID, listToolsResult{Tools: s.tools()})

	case "tools/call":
		return s.callTool(req)

	default:
		if req.IsNotification() {
			return nil
		}
		log.Printf("mcp: unknown method %q", req.Method)
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeMethodNotFound, "method not found: "+req.Method))
	}
}

// tools lists everything this server currently exposes.
func (s *Server) tools() []Tool {
	return []Tool{
		{
			Name:        "ping",
			Description: "Health check: returns pong.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "list_datasets",
			Description: "Lists the registered datasets with their description, origin, and row count.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

func (s *Server) callTool(req *jsonrpc.Request) *jsonrpc.Response {
	var params callToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeInvalidParams, "invalid params: "+err.Error()))
	}

	switch params.Name {
	case "ping":
		return jsonrpc.NewResult(req.ID, callToolResult{
			Content: []TextContent{{Type: "text", Text: "pong"}},
		})
	case "list_datasets":
		return s.listDatasets(req.ID)
	default:
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeInvalidParams, "unknown tool: "+params.Name))
	}
}

type datasetSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Origin      string `json:"origin"`
	RowCount    int64  `json:"row_count"`
}

func (s *Server) listDatasets(id *json.RawMessage) *jsonrpc.Response {
	summaries := make([]datasetSummary, 0, len(s.cat.Datasets))
	for _, ds := range s.cat.Datasets {
		rows, err := data.RowCount(s.cfg.DataDir, ds.File)
		if err != nil {
			log.Printf("mcp: list_datasets: %v", err)
			return jsonrpc.NewErrorResponse(id, jsonrpc.NewError(jsonrpc.CodeInternalError, "failed to read dataset "+ds.Name+": "+err.Error()))
		}
		summaries = append(summaries, datasetSummary{
			Name:        ds.Name,
			Description: ds.Description,
			Origin:      ds.Origin,
			RowCount:    rows,
		})
	}

	body, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return jsonrpc.NewErrorResponse(id, jsonrpc.NewError(jsonrpc.CodeInternalError, "failed to encode result: "+err.Error()))
	}

	return jsonrpc.NewResult(id, callToolResult{
		Content: []TextContent{{Type: "text", Text: string(body)}},
	})
}
