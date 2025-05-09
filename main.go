package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer("XMLUI", "0.1.0")

	xmluiDir := os.Args[1] // Get from CLI argument
	userHome, _ := os.UserHomeDir()

	listComponentsTool, listComponentsHandler := NewListComponentsTool(xmluiDir)
	s.AddTool(listComponentsTool, listComponentsHandler)

	componentDocsTool, componentDocsHandler := NewComponentDocsTool(xmluiDir)
	s.AddTool(componentDocsTool, componentDocsHandler)

	searchDocsTool, searchDocsHandler := NewSearchTool(xmluiDir)
	s.AddTool(searchDocsTool, searchDocsHandler)

	metadataTool, metadataHandler := NewMetadataTool()
	s.AddTool(metadataTool, metadataHandler)

	readFileTool, readFileHandler := NewReadFileTool(xmluiDir)
	s.AddTool(readFileTool, readFileHandler)

	exampleRoots := []string{
		filepath.Join(userHome, "xmlui-hn"),
		filepath.Join(userHome, "xmlui-invoice"),
		filepath.Join(userHome, "xmlui-cms"),
		filepath.Join(userHome, "xmlui-hn"),
		filepath.Join(userHome, "xmlui-hubspot"),
		filepath.Join(userHome, "xmlui-mastodon"),
	}

    examplesTool, examplesHandler := NewExamplesTool(exampleRoots)
	s.AddTool(examplesTool, examplesHandler)


	// Start the server using stdio transport
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
