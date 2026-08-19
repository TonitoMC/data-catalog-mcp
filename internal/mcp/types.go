package mcp

// Minimal subset of the MCP wire types — just enough for the
// initialize / tools/list / tools/call round trip.

const protocolVersion = "2024-11-05"

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    capability `json:"capabilities"`
	ServerInfo      serverInfo `json:"serverInfo"`
}

type capability struct {
	Tools map[string]any `json:"tools"`
}

// Tool describes one callable tool for tools/list.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type listToolsResult struct {
	Tools []Tool `json:"tools"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// TextContent is the simplest MCP content block.
type TextContent struct {
	Type string `json:"type"` // always "text"
	Text string `json:"text"`
}

type callToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}
