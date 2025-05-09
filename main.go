package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer("XMLUI", "0.1.0")

	homeDir := os.Args[1] // Get from CLI argument

	listComponentsTool, listComponentsHandler := NewListComponentsTool(homeDir)
	s.AddTool(listComponentsTool, listComponentsHandler)

	componentDocsTool, componentDocsHandler := NewComponentDocsTool(homeDir)
	s.AddTool(componentDocsTool, componentDocsHandler)

	searchDocsTool, searchDocsHandler := NewSearchTool(homeDir)
	s.AddTool(searchDocsTool, searchDocsHandler)

	metadataTool, metadataHandler := NewMetadataTool()
	s.AddTool(metadataTool, metadataHandler)

	readFileTool, readFileHandler := NewReadFileTool(homeDir)
	s.AddTool(readFileTool, readFileHandler)


	// Start the server using stdio transport
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
