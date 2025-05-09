package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer("XMLUI", "0.1.0")

	docsDir := os.Args[1] // Get from CLI argument

	listComponentsTool, listComponentsHandler := NewListComponentsTool(docsDir)
	s.AddTool(listComponentsTool, listComponentsHandler)


	xmluiDocsTool, xmluiDocsHandler := NewXmluiDocsTool(docsDir)
	s.AddTool(xmluiDocsTool, xmluiDocsHandler)

	// Start the server using stdio transport
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

// Handler function for the "hello_world" tool
func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, ok := request.Params.Arguments["name"].(string)
	if !ok {
		return nil, errors.New("name must be a string")
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}
