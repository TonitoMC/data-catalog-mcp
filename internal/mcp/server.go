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
	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
	"github.com/tonitomc/data-catalog-mcp/internal/tools"
)

// Server dispatches JSON-RPC requests to MCP method handlers.
type Server struct {
	tools map[string]Tool
}

// New builds a Server from the given config, loading the catalog up front
// so a bad catalog.yaml fails fast at startup rather than on first call.
func New(cfg config.Config) (*Server, error) {
	cat, err := catalog.Load(cfg.CatalogPath)
	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
	return &Server{tools: newRegistry(cfg, cat)}, nil
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
		return jsonrpc.NewResult(req.ID, listToolsResult{Tools: s.listTools()})

	case "tools/call":
		return s.callTool(req)

	default:
		if req.IsNotification() {
			return nil
		}
		log.Printf("mcp: unknown method %q", req.Method)
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeMethodNotFound, "method not found: "+req.Method, nil))
	}
}

// listTools projects the registry into the wire shape tools/list returns.
func (s *Server) listTools() []ToolInfo {
	infos := make([]ToolInfo, 0, len(s.tools))
	for _, t := range s.tools {
		infos = append(infos, ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return infos
}

func (s *Server) callTool(req *jsonrpc.Request) *jsonrpc.Response {
	var params callToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeInvalidParams, "invalid params: "+err.Error(), nil))
	}

	t, ok := s.tools[params.Name]
	if !ok {
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.NewError(jsonrpc.CodeInvalidParams, "unknown tool: "+params.Name, nil))
	}

	result, err := t.Call(params.Arguments)
	if err != nil {
		return toolErrorResponse(req.ID, params.Name, err)
	}
	return jsonResult(req.ID, result)
}

// toolErrorResponse maps a Tool.Call error to a JSON-RPC error: bad
// arguments and unresolvable references (ErrInvalidArgument,
// tools.ErrDatasetNotFound, ...) become "invalid params"; anything else is
// treated as an internal failure and logged.
func toolErrorResponse(id *json.RawMessage, toolName string, err error) *jsonrpc.Response {
	if errors.Is(err, ErrInvalidArgument) || errors.Is(err, tools.ErrDatasetNotFound) || errors.Is(err, tools.ErrColumnNotFound) {
		return jsonrpc.NewErrorResponse(id, jsonrpc.NewError(jsonrpc.CodeInvalidParams, err.Error(), map[string]string{
			"tool": toolName,
		}))
	}
	log.Printf("mcp: %s: %v", toolName, err)
	return jsonrpc.NewErrorResponse(id, jsonrpc.NewError(jsonrpc.CodeInternalError, "tool call failed", map[string]string{
		"tool":  toolName,
		"cause": err.Error(),
	}))
}

// jsonResult wraps v as a tool's text content result. A string result is
// used verbatim (e.g. ping's "pong"); anything else is marshaled as
// indented JSON.
func jsonResult(id *json.RawMessage, v any) *jsonrpc.Response {
	text, ok := v.(string)
	if !ok {
		body, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.NewError(jsonrpc.CodeInternalError, "failed to encode result: "+err.Error(), nil))
		}
		text = string(body)
	}
	return jsonrpc.NewResult(id, callToolResult{
		Content: []TextContent{{Type: "text", Text: text}},
	})
}
