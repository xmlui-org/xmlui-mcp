package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	mcpserver "xmlui-mcp/server"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Prompt API structures
type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PromptContent struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Messages    []mcp.PromptMessage `json:"messages"`
}

type PromptHandler func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// Session management structures
type SessionContext struct {
	ID              string              `json:"id"`
	InjectedPrompts []string            `json:"injected_prompts"`
	LastActivity    time.Time           `json:"last_activity"`
	Context         []mcp.PromptMessage `json:"context"`
}

type SessionManager struct {
	sessions map[string]*SessionContext
	mutex    sync.RWMutex
}

type InjectPromptRequest struct {
	SessionID  string `json:"session_id"`
	PromptName string `json:"prompt_name"`
}

type InjectPromptResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Content []mcp.PromptMessage `json:"content,omitempty"`
}

// Startup information structure
type StartupInfo struct {
	Type       string         `json:"type"`
	Prompts    []PromptInfo   `json:"prompts"`
	Tools      []ToolInfo     `json:"tools"`
	XmluiRules XmluiRulesInfo `json:"xmlui_rules"`
}

type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

type XmluiRulesInfo struct {
	Description string `json:"description"`
	Content     string `json:"content"`
}

func getToolInfo(tool mcp.Tool) ToolInfo {
	toolInfo := ToolInfo{
		Name:        tool.Name,
		Description: tool.Description,
	}

	if len(tool.InputSchema.Properties) > 0 {
		toolInfo.InputSchema = make(map[string]interface{})
		for name, prop := range tool.InputSchema.Properties {
			// Check if this property is required
			required := false
			for _, req := range tool.InputSchema.Required {
				if req == name {
					required = true
					break
				}
			}

			// Create property info
			propInfo := map[string]interface{}{
				"required": required,
			}

			// Extract description if present
			if propMap, ok := prop.(map[string]interface{}); ok {
				if d, ok := propMap["description"].(string); ok {
					propInfo["description"] = d
				}
				if t, ok := propMap["type"].(string); ok {
					propInfo["type"] = t
				}
			}

			toolInfo.InputSchema[name] = propInfo
		}
	}

	return toolInfo
}

func getXmluiRulesContent(handler PromptHandler) string {
	ctx := context.Background()
	request := mcp.GetPromptRequest{}

	result, err := handler(ctx, request)
	if err != nil {
		return fmt.Sprintf("Error getting xmlui_rules: %v", err)
	}

	var content strings.Builder
	for _, message := range result.Messages {
		switch textContent := message.Content.(type) {
		case *mcp.TextContent:
			content.WriteString(textContent.Text)
		case mcp.TextContent:
			content.WriteString(textContent.Text)
		}
	}
	return content.String()
}

func printStartupInfo(prompts []mcp.Prompt, tools []mcp.Tool, xmluiRulesHandler PromptHandler) {
	// Convert prompts to PromptInfo
	var promptInfos []PromptInfo
	for _, prompt := range prompts {
		promptInfos = append(promptInfos, PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
		})
	}

	// Convert tools to ToolInfo
	var toolInfos []ToolInfo
	for _, tool := range tools {
		toolInfos = append(toolInfos, getToolInfo(tool))
	}

	// Create startup info
	startupInfo := StartupInfo{
		Type:    "xmlui_mcp_startup",
		Prompts: promptInfos,
		Tools:   toolInfos,
		XmluiRules: XmluiRulesInfo{
			Description: "Essential rules and guidelines for XMLUI development",
			Content:     getXmluiRulesContent(xmluiRulesHandler),
		},
	}

	// Output as JSON to stderr to avoid interfering with MCP protocol
	jsonData, err := json.MarshalIndent(startupInfo, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling startup info: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s\n", string(jsonData))
}

// Session management methods
func (sm *SessionManager) GetOrCreateSession(id string) *SessionContext {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[id]; exists {
		session.LastActivity = time.Now()
		return session
	}

	session := &SessionContext{
		ID:              id,
		InjectedPrompts: []string{},
		LastActivity:    time.Now(),
		Context:         []mcp.PromptMessage{},
	}
	sm.sessions[id] = session
	return session
}

func (sm *SessionManager) InjectPrompt(sessionID, promptName string, promptHandlers map[string]PromptHandler) (*InjectPromptResponse, error) {
	// Get the prompt handler first (outside of lock)
	handler, exists := promptHandlers[promptName]
	if !exists {
		return &InjectPromptResponse{
			Success: false,
			Message: "Prompt not found",
		}, nil
	}

	// Call the handler to get content (outside of lock)
	ctx := context.Background()
	request := mcp.GetPromptRequest{}

	result, err := handler(ctx, request)
	if err != nil {
		return &InjectPromptResponse{
			Success: false,
			Message: "Error retrieving prompt content",
		}, err
	}

	// Now acquire lock and update session
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Get or create session (now safe since we already have the lock)
	var session *SessionContext
	if existingSession, exists := sm.sessions[sessionID]; exists {
		session = existingSession
		session.LastActivity = time.Now()
	} else {
		session = &SessionContext{
			ID:              sessionID,
			InjectedPrompts: []string{},
			LastActivity:    time.Now(),
			Context:         []mcp.PromptMessage{},
		}
		sm.sessions[sessionID] = session
	}

	// Add to session context
	session.Context = append(session.Context, result.Messages...)
	session.InjectedPrompts = append(session.InjectedPrompts, promptName)
	session.LastActivity = time.Now()

	return &InjectPromptResponse{
		Success: true,
		Message: fmt.Sprintf("Prompt '%s' injected into session '%s'", promptName, sessionID),
		Content: result.Messages,
	}, nil
}

func getCurrentDir() string {
	// Try to get the directory of the executable
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}

	// Fallback to current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
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

	// Store prompts and their handlers for API access
	var promptsList []mcp.Prompt
	var promptHandlers map[string]PromptHandler = make(map[string]PromptHandler)

	// Define the xmlui_rules prompt handler once
	xmluiRulesHandler := func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

	// Create the xmlui_rules prompt
	xmluiRulesPrompt := mcp.NewPrompt("xmlui_rules",
		mcp.WithPromptDescription("Essential rules and guidelines for XMLUI development"))

	// Store in our lists for API access
	promptsList = append(promptsList, xmluiRulesPrompt)
	promptHandlers["xmlui_rules"] = xmluiRulesHandler

	// Register with MCP server
	s.AddPrompt(xmluiRulesPrompt, xmluiRulesHandler)

	// Initialize analytics
	analyticsFile := filepath.Join(getCurrentDir(), "xmlui-mcp-analytics.json")
	mcpserver.InitializeAnalytics(analyticsFile)

	// Initialize session manager
	var sessionManager = &SessionManager{
		sessions: make(map[string]*SessionContext),
	}

	// Store tools for the /tools endpoint
	var toolsList []mcp.Tool

	listComponentsTool, listComponentsHandler := mcpserver.NewListComponentsTool(xmluiDir)
	s.AddTool(listComponentsTool, mcpserver.WithAnalytics("xmlui_list_components", listComponentsHandler))
	toolsList = append(toolsList, listComponentsTool)

	componentDocsTool, componentDocsHandler := mcpserver.NewComponentDocsTool(xmluiDir)
	s.AddTool(componentDocsTool, mcpserver.WithAnalytics("xmlui_component_docs", componentDocsHandler))
	toolsList = append(toolsList, componentDocsTool)

	searchDocsTool, searchDocsHandler := mcpserver.NewSearchTool(xmluiDir)
	s.AddTool(searchDocsTool, mcpserver.WithSearchAnalytics("xmlui_search", searchDocsHandler))
	toolsList = append(toolsList, searchDocsTool)

	readFileTool, readFileHandler := mcpserver.NewReadFileTool(xmluiDir)
	s.AddTool(readFileTool, mcpserver.WithAnalytics("xmlui_read_file", readFileHandler))
	toolsList = append(toolsList, readFileTool)

	examplesTool, examplesHandler := mcpserver.NewExamplesTool(exampleRoots)
	s.AddTool(examplesTool, mcpserver.WithSearchAnalytics("xmlui_examples", examplesHandler))
	toolsList = append(toolsList, examplesTool)

	listHowtoTool, listHowtoHandler := mcpserver.NewListHowtoTool(xmluiDir)
	s.AddTool(listHowtoTool, mcpserver.WithAnalytics("xmlui_list_howto", listHowtoHandler))
	toolsList = append(toolsList, listHowtoTool)

	searchHowtoTool, searchHowtoHandler := mcpserver.NewSearchHowtoTool(xmluiDir)
	s.AddTool(searchHowtoTool, mcpserver.WithSearchAnalytics("xmlui_search_howto", searchHowtoHandler))
	toolsList = append(toolsList, searchHowtoTool)

	// Add prompt injection tool
	injectPromptTool := mcp.NewTool("xmlui_inject_prompt",
		mcp.WithDescription("Inject a prompt into the current session context for guidance"),
	)

	injectPromptTool.InputSchema = mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"prompt_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the prompt to inject (e.g., 'xmlui_rules')",
				"default":     "xmlui_rules",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID (optional, defaults to 'default')",
				"default":     "default",
			},
		},
		Required: []string{"prompt_name"},
	}

	injectPromptHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		promptName := "xmlui_rules" // default
		sessionID := "default"      // default

		if request.Params.Arguments != nil {
			if name, ok := request.Params.Arguments["prompt_name"].(string); ok && name != "" {
				promptName = name
			}
			if id, ok := request.Params.Arguments["session_id"].(string); ok && id != "" {
				sessionID = id
			}
		}

		// Use the session manager to inject
		response, err := sessionManager.InjectPrompt(sessionID, promptName, promptHandlers)
		if err != nil {
			return mcp.NewToolResultError("Error injecting prompt: " + err.Error()), nil
		}

		if response.Success {
			return mcp.NewToolResultText(fmt.Sprintf("✅ Successfully injected '%s' prompt into session '%s'. The guidelines are now active in your context.", promptName, sessionID)), nil
		} else {
			return mcp.NewToolResultError("❌ Failed to inject prompt: " + response.Message), nil
		}
	}

	s.AddTool(injectPromptTool, mcpserver.WithAnalytics("xmlui_inject_prompt", injectPromptHandler))
	toolsList = append(toolsList, injectPromptTool)

	// Add prompt listing tool
	listPromptsTool := mcp.NewTool("xmlui_list_prompts",
		mcp.WithDescription("Lists all available prompts that can be injected into session context"),
	)

	listPromptsHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var out strings.Builder
		out.WriteString("Available prompts:\n\n")

		for _, prompt := range promptsList {
			out.WriteString(fmt.Sprintf("- **%s**: %s\n", prompt.Name, prompt.Description))
		}

		out.WriteString("\nUse xmlui_get_prompt to view content or xmlui_inject_prompt to inject into context.")
		return mcp.NewToolResultText(out.String()), nil
	}

	s.AddTool(listPromptsTool, mcpserver.WithAnalytics("xmlui_list_prompts", listPromptsHandler))
	toolsList = append(toolsList, listPromptsTool)

	// Add prompt content retrieval tool
	getPromptTool := mcp.NewTool("xmlui_get_prompt",
		mcp.WithDescription("Retrieves the content of a specific prompt for review"),
	)

	getPromptTool.InputSchema = mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"prompt_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the prompt to retrieve (e.g., 'xmlui_rules')",
			},
		},
		Required: []string{"prompt_name"},
	}

	getPromptHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract prompt name
		promptName := ""
		if request.Params.Arguments != nil {
			if name, ok := request.Params.Arguments["prompt_name"].(string); ok {
				promptName = name
			}
		}

		if promptName == "" {
			return mcp.NewToolResultError("prompt_name parameter is required"), nil
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
			return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", promptName)), nil
		}

		// Get the prompt handler
		handler, exists := promptHandlers[promptName]
		if !exists {
			return mcp.NewToolResultError("Prompt handler not found"), nil
		}

		// Call the handler to get content
		result, err := handler(ctx, mcp.GetPromptRequest{})
		if err != nil {
			return mcp.NewToolResultError("Error retrieving prompt content: " + err.Error()), nil
		}

		// Format the output
		var out strings.Builder
		out.WriteString(fmt.Sprintf("# %s\n\n", foundPrompt.Name))
		out.WriteString(fmt.Sprintf("**Description:** %s\n\n", foundPrompt.Description))
		out.WriteString("**Content:**\n\n")

		for _, message := range result.Messages {
			// Extract text content
			switch content := message.Content.(type) {
			case *mcp.TextContent:
				out.WriteString(content.Text)
				out.WriteString("\n")
			case mcp.TextContent:
				out.WriteString(content.Text)
				out.WriteString("\n")
			}
		}

		return mcp.NewToolResultText(out.String()), nil
	}

	s.AddTool(getPromptTool, mcpserver.WithAnalytics("xmlui_get_prompt", getPromptHandler))
	toolsList = append(toolsList, getPromptTool)

	// Print startup information as JSON to stderr
	printStartupInfo(promptsList, toolsList, xmluiRulesHandler)

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

		// GET /session/{id} - Get session context
		mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Extract session ID from URL path
			sessionID := strings.TrimPrefix(r.URL.Path, "/session/")
			if sessionID == "" {
				http.Error(w, "Session ID required", http.StatusBadRequest)
				return
			}

			session := sessionManager.GetOrCreateSession(sessionID)
			json.NewEncoder(w).Encode(session)
		})

		// POST /session/context - Inject prompt into session
		mux.HandleFunc("/session/context", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			if r.Method != "POST" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var req InjectPromptRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			if req.SessionID == "" {
				req.SessionID = "default" // Use default session if not specified
			}

			if req.PromptName == "" {
				http.Error(w, "Prompt name required", http.StatusBadRequest)
				return
			}

			response, err := sessionManager.InjectPrompt(req.SessionID, req.PromptName, promptHandlers)
			if err != nil {
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(response)
		})

		// Add analytics endpoints
		mux.HandleFunc("/analytics/summary", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			summary := mcpserver.GetAnalyticsSummary()
			json.NewEncoder(w).Encode(summary)
		})

		addr := ":" + *port
		mcpserver.WriteDebugLog("Starting HTTP server on port %s\n", *port)
		mcpserver.WriteDebugLog("SSE endpoint: http://localhost%s/sse\n", addr)
		mcpserver.WriteDebugLog("Message endpoint: http://localhost%s/message\n", addr)
		mcpserver.WriteDebugLog("Tools endpoint: http://localhost%s/tools\n", addr)
		mcpserver.WriteDebugLog("Prompts list endpoint: http://localhost%s/prompts\n", addr)
		mcpserver.WriteDebugLog("Specific prompt endpoint: http://localhost%s/prompts/{name}\n", addr)
		mcpserver.WriteDebugLog("Session context endpoint: http://localhost%s/session/{id}\n", addr)
		mcpserver.WriteDebugLog("Inject prompt endpoint: http://localhost%s/session/context\n", addr)
		mcpserver.WriteDebugLog("Analytics summary endpoint: http://localhost%s/analytics/summary\n", addr)

		if err := http.ListenAndServe(addr, mux); err != nil {
			mcpserver.WriteDebugLog("HTTP server error: %v\n", err)
			os.Exit(1)
		}

	} else {
		// Stdio mode with graceful shutdown

		// Set up signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start server in a goroutine
		serverDone := make(chan error, 1)
		go func() {
			serverDone <- server.ServeStdio(s)
		}()

		// Wait for either server error or signal
		select {
		case err := <-serverDone:
			// Server finished (normally or with error)
			if err != nil {
				mcpserver.WriteDebugLog("Server error: %v\n", err)
				os.Exit(1)
			}
		case <-sigChan:
			mcpserver.WriteDebugLog("Received shutdown signal, initiating graceful shutdown\n")

			// Wait for server to shutdown gracefully with timeout
			select {
			case err := <-serverDone:
				if err != nil {
					mcpserver.WriteDebugLog("Server shutdown with error: %v\n", err)
				} else {
					mcpserver.WriteDebugLog("Server shutdown complete\n")
				}
			case <-time.After(5 * time.Second):
				mcpserver.WriteDebugLog("Server shutdown timeout, forcing exit\n")
				os.Exit(1)
			}
		}
	}
}
