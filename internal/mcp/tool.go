package mcp

// Tool is one callable MCP tool. Implementations are thin adapters: they
// hold whatever a tool needs to run (config, the catalog, ...) and delegate
// the actual work to internal/tools, which knows nothing about MCP or
// JSON-RPC. Call takes decoded JSON arguments and returns a plain Go value
// to be marshaled as the result, or an error to be mapped onto a JSON-RPC
// error response.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Call(args map[string]any) (any, error)
}
