package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/tonitomc/data-catalog-mcp/internal/client"
	"github.com/tonitomc/data-catalog-mcp/internal/llm"
)

func main() {
	_ = godotenv.Load()
	llmURL := os.Getenv("LLM_API_URL")
	llmModel := os.Getenv("LLM_MODEL")
	model := llm.New(llmURL, llmModel, os.Getenv("LLM_API_KEY"))

	cfg, err := client.LoadConfig("client.json")
	if err != nil { panic(err) }
	router, err := client.NewRouter(cfg)
	if err != nil { panic(err) }
	defer router.Close()

	var tools []llm.Tool
	names := map[string]string{}
	for _, server := range router.Servers() {
		ts, err := router.ListTools(server)
		if err != nil { panic(err) }
		for _, t := range ts {
			q := server + "__" + t.Name
			names[q] = server
			tools = append(tools, llm.Tool{Type:"function", Function: llm.ToolFunction{Name:q, Description:t.Description, Parameters:t.InputSchema}})
		}
	}

	history := []llm.Message{
		{Role:"system", Content:"You are a helpful assistant with access to tools for a data catalog, the local filesystem, and git. Use them when they help answer the user's question."},
		{Role:"user", Content:"Do we have any data on employees?"},
	}

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		fmt.Println("=== calling model ===")
		reply, err := model.Chat(ctx, history, tools)
		if err != nil { fmt.Println("ERR:", err); return }
		fmt.Printf("reply: %+v\n", reply)
		history = append(history, reply)
		if len(reply.ToolCalls) == 0 {
			fmt.Println("DONE:", reply.Content)
			return
		}
		for _, tc := range reply.ToolCalls {
			server := names[tc.Function.Name]
			toolName := tc.Function.Name[len(server)+2:]
			fmt.Println("calling tool", server, toolName, tc.Function.Arguments)
			res, err := router.CallTool(server, toolName, tc.Function.Arguments)
			if err != nil { fmt.Println("tool err:", err) }
			fmt.Println("tool result:", res)
			history = append(history, llm.Message{Role:"tool", Content: res, ToolCallID: tc.ID})
		}
	}
}
