package mcp

// pingTool is a health check; it takes no arguments.
type pingTool struct{}

func (pingTool) Name() string        { return "ping" }
func (pingTool) Description() string { return "Health check: returns pong." }
func (pingTool) InputSchema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}
func (pingTool) Call(args map[string]any) (any, error) {
	return "pong", nil
}
