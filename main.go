package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: ./xmlui-mcp <xmluiDir> [exampleRoot] [comma-separated-exampleDirs]")
		os.Exit(1)
	}

	xmluiDir := os.Args[1]
	exampleRoot := ""
	exampleDirs := []string{}
	exampleRoots := []string{}

	// Optional arg 2: example root
	if len(os.Args) >= 3 {
		exampleRoot = os.Args[2]
	}

	// Optional arg 3: comma-separated subdirs
	if len(os.Args) >= 4 {
		exampleDirs = strings.Split(os.Args[3], ",")
	}

	// Construct exampleRoots if exampleRoot and dirs are present
	if exampleRoot != "" && len(exampleDirs) > 0 {
		for _, d := range exampleDirs {
			trimmed := strings.TrimSpace(d)
			if trimmed != "" {
				exampleRoots = append(exampleRoots, filepath.Join(exampleRoot, trimmed))
			}
		}
	}

	// Start MCP server
	s := server.NewMCPServer("XMLUI", "0.1.0")

	// Add tools
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

	examplesTool, examplesHandler := NewExamplesTool(exampleRoots)
	s.AddTool(examplesTool, examplesHandler)

	fmt.Fprintf(os.Stderr, "✅ Registered tool: %s\n", examplesTool.Name)

	// Launch
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
