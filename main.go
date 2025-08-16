//go:build server
// +build server

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Prompt API structures
type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PromptContent struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Messages    []mcp.PromptMessage   `json:"messages"`
}

type PromptHandler func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

func printToolRegistration(tool mcp.Tool) {
	fmt.Fprintf(os.Stderr, "%s\n", tool.Name)
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

func printPromptRegistration(prompt mcp.Prompt) {
	fmt.Fprintf(os.Stderr, "PROMPT: %s\n", prompt.Name)
	fmt.Fprintf(os.Stderr, " %s\n", prompt.Description)
	fmt.Fprintln(os.Stderr)
}

func main() {
	// Define command-line flags
	var (
		httpMode = flag.Bool("http", false, "Run in HTTP mode instead of stdio")
		port     = flag.String("port", "8080", "Port to listen on in HTTP mode")
	)

	// Parse flags but allow positional arguments
	flag.Parse()

	// Get positional arguments after flags
	args := flag.Args()

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ./xmlui-mcp [--http] [--port PORT] <xmluiDir> [exampleRoot] [comma-separated-exampleDirs]")
		fmt.Fprintln(os.Stderr, "  --http: Run in HTTP mode (default: stdio mode)")
		fmt.Fprintln(os.Stderr, "  --port: Port to listen on in HTTP mode (default: 8080)")
		os.Exit(1)
	}

	xmluiDir := args[0]
	exampleRoot := ""
	exampleDirs := []string{}
	exampleRoots := []string{}

	// Optional arg 2: example root
	if len(args) >= 2 {
		exampleRoot = args[1]
	}

	// Optional arg 3: comma-separated subdirs
	if len(args) >= 3 {
		exampleDirs = strings.Split(args[2], ",")
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
	s := server.NewMCPServer("XMLUI", "0.1.0",
		server.WithPromptCapabilities(true),
	)

	// Add XMLUI development rules prompt
	xmluiRulesPrompt := mcp.NewPrompt("xmlui_rules",
		mcp.WithPromptDescription("Essential rules and guidelines for XMLUI development"))

	s.AddPrompt(xmluiRulesPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult(
			"XMLUI Development Rules and Guidelines",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(
					mcp.RoleUser,
					mcp.NewTextContent(`You are assisting with XMLUI development. Follow these essential rules:

1 don't write any code without my permission, always preview proposed changes, discuss, and only proceed with approval.

2 don't add any xmlui styling, let the theme and layout engine do its job

3 proceed in small increments, write the absolute minimum amount of xmlui markup necessary and no script if possible

4 do not invent any xmlui syntax. only use constructs for which you can find examples in the docs and sample apps. cite your sources.

5 never touch the dom. we only work within xmlui abstractions inside the <App> realm, with help from vars and functions defined on the window variable in index.html

6 keep complex functions and expressions out of xmlui, then can live in index.html or (if scoping requires) in code-behind

7 use the xmlui mcp server to list and show component docs but also search xmlui source, docs, and examples

8 always do the simplest thing possible

9 use a neutral tone. do not say "Perfect!" etc. in fact never use exclamation marks at all

10 when creating examples for live playgrounds, observe the conventions for ---app and ---comp

11 VStack is the default, don't use it unless necessary

12 always search XMLUI-related resources first and prioritize them over other sources

These rules ensure clean, maintainable XMLUI applications that follow best practices.`),
				),
			},
		), nil
	})

	printPromptRegistration(xmluiRulesPrompt)

	// Store prompts and their handlers for API access
	var promptsList []mcp.Prompt
	var promptHandlers map[string]PromptHandler = make(map[string]PromptHandler)

	// Register the xmlui_rules prompt
	promptsList = append(promptsList, xmluiRulesPrompt)
	promptHandlers["xmlui_rules"] = func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult(
			"XMLUI Development Rules and Guidelines",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(
					mcp.RoleUser,
					mcp.NewTextContent(`You are assisting with XMLUI development. Follow these essential rules:

1 don't write any code without my permission, always preview proposed changes, discuss, and only proceed with approval.

2 don't add any xmlui styling, let the theme and layout engine do its job

3 proceed in small increments, write the absolute minimum amount of xmlui markup necessary and no script if possible

4 do not invent any xmlui syntax. only use constructs for which you can find examples in the docs and sample apps. cite your sources.

5 never touch the dom. we only work within xmlui abstractions inside the <App> realm, with help from vars and functions defined on the window variable in index.html

6 keep complex functions and expressions out of xmlui, then can live in index.html or (if scoping requires) in code-behind

7 use the xmlui mcp server to list and show component docs but also search xmlui source, docs, and examples

8 always do the simplest thing possible

9 use a neutral tone. do not say "Perfect!" etc. in fact never use exclamation marks at all

10 when creating examples for live playgrounds, observe the conventions for ---app and ---comp

11 VStack is the default, don't use it unless necessary

12 always search XMLUI-related resources first and prioritize them over other sources

These rules ensure clean, maintainable XMLUI applications that follow best practices.`),
				),
			},
		), nil
	}

	// Store tools for the /tools endpoint
	var toolsList []mcp.Tool

	listComponentsTool, listComponentsHandler := NewListComponentsTool(xmluiDir)
	s.AddTool(listComponentsTool, listComponentsHandler)
	toolsList = append(toolsList, listComponentsTool)
	printToolRegistration(listComponentsTool)

	componentDocsTool, componentDocsHandler := NewComponentDocsTool(xmluiDir)
	s.AddTool(componentDocsTool, componentDocsHandler)
	toolsList = append(toolsList, componentDocsTool)
	printToolRegistration(componentDocsTool)

	searchDocsTool, searchDocsHandler := NewSearchTool(xmluiDir)
	s.AddTool(searchDocsTool, searchDocsHandler)
	toolsList = append(toolsList, searchDocsTool)
	printToolRegistration(searchDocsTool)

	readFileTool, readFileHandler := NewReadFileTool(xmluiDir)
	s.AddTool(readFileTool, readFileHandler)
	toolsList = append(toolsList, readFileTool)
	printToolRegistration(readFileTool)

	examplesTool, examplesHandler := NewExamplesTool(exampleRoots)
	s.AddTool(examplesTool, examplesHandler)
	toolsList = append(toolsList, examplesTool)
	printToolRegistration(examplesTool)

	listHowtoTool, listHowtoHandler := NewListHowtoTool(xmluiDir)
	s.AddTool(listHowtoTool, listHowtoHandler)
	toolsList = append(toolsList, listHowtoTool)
	printToolRegistration(listHowtoTool)

	searchHowtoTool, searchHowtoHandler := NewSearchHowtoTool(xmluiDir)
	s.AddTool(searchHowtoTool, searchHowtoHandler)
	toolsList = append(toolsList, searchHowtoTool)
	printToolRegistration(searchHowtoTool)

	// Launch based on mode
	if *httpMode {
		// HTTP mode
		sseServer := server.NewSSEServer(s)

		// Create a custom mux to add the /tools endpoint
		mux := http.NewServeMux()

		// Add the SSE server routes
		mux.Handle("/sse", sseServer)
		mux.Handle("/message", sseServer)

		// Add the /tools endpoint for VS Code toolset validation
		mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Convert to the format VS Code expects
			var toolList []map[string]string
			for _, tool := range toolsList {
				toolList = append(toolList, map[string]string{
					"name":        tool.Name,
					"description": tool.Description,
				})
			}

			json.NewEncoder(w).Encode(toolList)
		})

		// Add the /prompts endpoint to list all prompts
		mux.HandleFunc("/prompts", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Convert to API format
			var promptInfoList []PromptInfo
			for _, prompt := range promptsList {
				promptInfoList = append(promptInfoList, PromptInfo{
					Name:        prompt.Name,
					Description: prompt.Description,
				})
			}

			json.NewEncoder(w).Encode(promptInfoList)
		})

		// Add the /prompts/{name} endpoint to retrieve specific prompt
		mux.HandleFunc("/prompts/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Extract prompt name from URL path
			promptName := strings.TrimPrefix(r.URL.Path, "/prompts/")
			if promptName == "" {
				http.Error(w, "Prompt name required", http.StatusBadRequest)
				return
			}

			// Find the prompt
			var foundPrompt *mcp.Prompt
			for _, prompt := range promptsList {
				if prompt.Name == promptName {
					foundPrompt = &prompt
					break
				}
			}

			if foundPrompt == nil {
				http.Error(w, "Prompt not found", http.StatusNotFound)
				return
			}

			// Get the prompt handler
			handler, exists := promptHandlers[promptName]
			if !exists {
				http.Error(w, "Prompt handler not found", http.StatusInternalServerError)
				return
			}

			// Call the handler to get content
			ctx := context.Background()
			request := mcp.GetPromptRequest{}
			
			result, err := handler(ctx, request)
			if err != nil {
				http.Error(w, "Error retrieving prompt content", http.StatusInternalServerError)
				return
			}

			// Create response with prompt content
			promptContent := PromptContent{
				Name:        foundPrompt.Name,
				Description: foundPrompt.Description,
				Messages:    result.Messages,
			}

			json.NewEncoder(w).Encode(promptContent)
		})

		addr := ":" + *port
		fmt.Fprintf(os.Stderr, "Starting HTTP server on port %s\n", *port)
		fmt.Fprintf(os.Stderr, "SSE endpoint: http://localhost%s/sse\n", addr)
		fmt.Fprintf(os.Stderr, "Message endpoint: http://localhost%s/message\n", addr)
		fmt.Fprintf(os.Stderr, "Tools endpoint: http://localhost%s/tools\n", addr)
		fmt.Fprintf(os.Stderr, "Prompts list endpoint: http://localhost%s/prompts\n", addr)
		fmt.Fprintf(os.Stderr, "Specific prompt endpoint: http://localhost%s/prompts/{name}\n", addr)

		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Stdio mode (existing behavior)
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}
