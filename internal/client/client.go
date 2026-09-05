// Package client implements a barebones MCP client, over either of two
// transports: stdio (spawns the server as a subprocess) or plain HTTP
// (talks to a server already running somewhere, e.g. on Cloud Run). Both
// expose the same Client interface — initialize / tools/list / tools/call
// — so callers (Router, and everything built on it) don't care which
// transport a given server actually uses.
package client

const protocolVersion = "2024-11-05"

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      clientInfo     `json:"clientInfo"`
}

// ToolInfo describes one callable tool, as returned by tools/list.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// TextContent is the simplest MCP content block.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type listToolsResult struct {
	Tools []ToolInfo `json:"tools"`
}

type callToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Client talks MCP to a single server: the handshake plus the two calls
// every host needs. Implementations: stdioClient (Connect/ConnectWithEnv,
// stdio.go) and httpClient (ConnectHTTP, http_client.go).
type Client interface {
	// Initialize performs the MCP handshake: initialize request followed
	// by the notifications/initialized notification.
	Initialize() error

	// ListTools returns the server's tools/list result.
	ListTools() ([]ToolInfo, error)

	// CallTool invokes a tool by name and returns its concatenated text
	// content. If the server reports IsError, the text is still returned
	// alongside a non-nil error.
	CallTool(name string, args map[string]any) (string, error)

	// Close releases anything the transport holds open (a subprocess for
	// stdio; nothing for HTTP, which is stateless between calls).
	Close() error
}
