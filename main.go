package main

import (
	"fmt"
	"os"

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

	searchDocsTool, searchDocsHandler := NewSearchDocsTool(docsDir)
	s.AddTool(searchDocsTool, searchDocsHandler)


	// Start the server using stdio transport
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
