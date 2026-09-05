package mcp

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
)

// HTTPHandler serves MCP over plain HTTP: POST one JSON-RPC message, get
// one JSON-RPC response back. This is a deliberately minimal reading of
// MCP's Streamable HTTP transport — no SSE, no session IDs — sufficient
// because both ends of this connection are this repo's own code, and the
// dispatch logic (s.handle) is exactly what the stdio transport
// (Server.Run, see server.go) already calls. Adding this required no
// changes there.
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		var req jsonrpc.Request
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, jsonrpc.NewErrorResponse(nil, jsonrpc.NewError(jsonrpc.CodeParseError, "malformed JSON-RPC message: "+err.Error(), nil)))
			return
		}

		resp := s.handle(&req)
		if req.IsNotification() {
			// No reply expected. 204 rather than 200-with-empty-body so a
			// client never tries to JSON-decode nothing.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, resp)
	})
}

func writeJSON(w http.ResponseWriter, resp *jsonrpc.Response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
