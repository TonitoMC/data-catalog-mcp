// Command client is a barebones multi-server MCP client. It reads a
// config listing servers to connect to (same mcpServers schema as
// mcphost.json), spawns each as a subprocess, and routes tool calls to
// the right one by server name. See client.example.json for the shape.
//
// Usage:
//
//	client [-config path]                              # list every server's tools
//	client [-config path] call <server> <tool> [json]   # call one tool
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/tonitomc/data-catalog-mcp/internal/client"
)

func main() {
	configPath := flag.String("config", "client.json", "path to an mcpServers config file (see client.example.json)")
	flag.Parse()

	cfg, err := client.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	router, err := client.NewRouter(cfg)
	if err != nil {
		log.Fatalf("client: %v", err)
	}
	defer router.Close()

	args := flag.Args()
	if len(args) == 0 {
		listAll(router)
		return
	}
	if args[0] != "call" || len(args) < 3 {
		log.Fatal("usage: client [-config path] [call <server> <tool> [json-args]]")
	}
	callTool(router, args[1], args[2], args[3:])
}

func listAll(r *client.Router) {
	for _, server := range r.Servers() {
		tools, err := r.ListTools(server)
		if err != nil {
			log.Printf("client: %s: %v", server, err)
			continue
		}
		fmt.Printf("== %s ==\n", server)
		for _, t := range tools {
			fmt.Printf("  %-26s %s\n", t.Name, t.Description)
		}
	}
}

func callTool(r *client.Router, server, tool string, rest []string) {
	var args map[string]any
	if len(rest) > 0 {
		if err := json.Unmarshal([]byte(rest[0]), &args); err != nil {
			log.Fatalf("client: invalid json args: %v", err)
		}
	}

	result, err := r.CallTool(server, tool, args)
	if err != nil {
		log.Fatalf("client: %v", err)
	}
	fmt.Println(result)
}
