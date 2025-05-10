//go:build client
// +build client
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./xmlui-mcp-client \"serverPath xmluiDir [examplesDir repo1,repo2]\"")
		fmt.Println("Example ./xmlui-mcp-client \"~/xmlui-mcp/xmlui-mcp ~/xmlui ~ xmlui-hn,xmlui-mastodon,xmlui-invoice\"")
		os.Exit(1)
	}

	commandLine := os.Args[1]
	parts := strings.Fields(commandLine)
	cmd := parts[0]
	args := parts[1:]

	ctx := context.Background()

	c, err := client.NewStdioMCPClient(cmd, nil, args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to MCP server:", err)
		os.Exit(1)
	}
	defer c.Close()

	_, err = c.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to initialize session:", err)
		os.Exit(1)
	}

	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to list tools:", err)
		os.Exit(1)
	}

	toolMap := make(map[string]mcp.Tool)
	for _, t := range tools.Tools {
		toolMap[t.Name] = t
	}

	fmt.Println("\nConnected to server with tools:", toolNames(tools.Tools))
	fmt.Println("Type 'help' for commands, 'quit' to exit.")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		HistoryFile:     "/tmp/xmlui-client-history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistoryLimit:    500,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to initialize readline:", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "quit" {
			break
		}
		response := handleQuery(ctx, c, toolMap, line)
		fmt.Println("\n" + response)
	}
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

func handleQuery(ctx context.Context, c *client.Client, toolMap map[string]mcp.Tool, input string) string {
	if input == "" {
		return "Please enter a command or type 'help'."
	}
	if input == "help" {
		return `Available commands:
  list              - List all XMLUI components
  docs <name>       - Docs for a component
  search <term>     - Search XMLUI code/docs
  read <path>       - Read a file
  examples <query>  - Search usage examples
  meta              - Tool help
  help              - Show this help
  quit              - Quit`
	}

	parts := strings.Fields(input)
	cmd := parts[0]
	arg := strings.Join(parts[1:], " ")

	toolName := ""
	args := map[string]any{}

	switch cmd {
	case "list":
		toolName = "xmlui_list_components"
	case "docs":
		toolName = "xmlui_component_docs"
		args["component"] = capitalize(arg)
	case "search":
		toolName = "xmlui_search"
		args["query"] = arg
	case "read":
		toolName = "xmlui_read_file"
		args["path"] = arg
	case "examples":
		toolName = "xmlui_examples"
		args["query"] = arg
	case "meta":
		toolName = "xmlui_metadata"
	default:
		return "Unrecognized command. Type 'help'."
	}

	result, _ := c.CallTool(ctx, mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      toolName,
			Arguments: args,
			Meta:      nil,
		},
	})

	var out []string
	for _, c := range result.Content {
		out = append(out, fmt.Sprintf("%v", c))
	}
	return strings.Join(out, "\n")
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
