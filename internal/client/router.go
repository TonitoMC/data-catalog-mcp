package client

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ServerConfig describes how to launch one MCP server. It mirrors the
// mcpServers schema already used by mcphost.json / mcphost.example.json
// in this repo, so the same config shape works for both.
type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Config is a named set of servers to connect to.
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// LoadConfig reads and parses a Config from a JSON file at path.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("client: read config %q: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("client: parse config %q: %w", path, err)
	}
	return cfg, nil
}

// Router owns one Client per configured server and routes tool calls to
// the right one by server name.
type Router struct {
	clients map[string]*Client
}

// NewRouter connects to and initializes every server in cfg. If any
// server fails to connect or initialize, servers already brought up are
// closed before returning the error.
func NewRouter(cfg Config) (*Router, error) {
	clients := make(map[string]*Client, len(cfg.MCPServers))
	for name, sc := range cfg.MCPServers {
		c, err := ConnectWithEnv(sc.Command, sc.Args, sc.Env)
		if err != nil {
			closeAll(clients)
			return nil, fmt.Errorf("client: connect %q: %w", name, err)
		}
		if err := c.Initialize(); err != nil {
			_ = c.Close()
			closeAll(clients)
			return nil, fmt.Errorf("client: initialize %q: %w", name, err)
		}
		clients[name] = c
	}
	return &Router{clients: clients}, nil
}

func closeAll(clients map[string]*Client) {
	for _, c := range clients {
		_ = c.Close()
	}
}

// Close closes every underlying server connection.
func (r *Router) Close() {
	closeAll(r.clients)
}

// Servers returns the configured server names, sorted.
func (r *Router) Servers() []string {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListTools returns the named server's tools.
func (r *Router) ListTools(server string) ([]ToolInfo, error) {
	c, ok := r.clients[server]
	if !ok {
		return nil, fmt.Errorf("client: unknown server %q", server)
	}
	return c.ListTools()
}

// CallTool routes a tool call to the named server.
func (r *Router) CallTool(server, tool string, args map[string]any) (string, error) {
	c, ok := r.clients[server]
	if !ok {
		return "", fmt.Errorf("client: unknown server %q", server)
	}
	return c.CallTool(tool, args)
}
