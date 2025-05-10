package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func printToolRegistration(tool mcp.Tool) {
	fmt.Fprintf(os.Stderr, "Registered tool: %s\n", tool.Name)
	fmt.Fprintf(os.Stderr, " %s\n", tool.Description)

	if len(tool.InputSchema.Properties) > 0 {
		fmt.Fprintf(os.Stderr, " Input schema:\n")
		for name, prop := range tool.InputSchema.Properties {
			required := ""
			for _, req := range tool.InputSchema.Required {
				if req == name {
					required = "(required)"
					break
				}
			}

			// Safely extract description if present
			desc := "(no description)"
			if propMap, ok := prop.(map[string]interface{}); ok {
				if d, ok := propMap["description"].(string); ok {
					desc = d
				}
			}

			fmt.Fprintf(os.Stderr, "   - %s %s: %s\n", name, required, desc)
		}
	}

	fmt.Fprintln(os.Stderr)
}

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

	listComponentsTool, listComponentsHandler := NewListComponentsTool(xmluiDir)
	s.AddTool(listComponentsTool, listComponentsHandler)
	printToolRegistration(listComponentsTool)

	componentDocsTool, componentDocsHandler := NewComponentDocsTool(xmluiDir)
	s.AddTool(componentDocsTool, componentDocsHandler)
	printToolRegistration(componentDocsTool)

	searchDocsTool, searchDocsHandler := NewSearchTool(xmluiDir)
	s.AddTool(searchDocsTool, searchDocsHandler)
	printToolRegistration(searchDocsTool)

	metadataTool, metadataHandler := NewMetadataTool()
	s.AddTool(metadataTool, metadataHandler)
	printToolRegistration(metadataTool)

	readFileTool, readFileHandler := NewReadFileTool(xmluiDir)
	s.AddTool(readFileTool, readFileHandler)
	printToolRegistration(readFileTool)

	examplesTool, examplesHandler := NewExamplesTool(exampleRoots)
	s.AddTool(examplesTool, examplesHandler)
	printToolRegistration(examplesTool)


	// Launch
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
