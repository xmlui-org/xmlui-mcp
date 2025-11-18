package xmluimcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xmlui-org/mcpsvr/xmluimcp/common"
	"github.com/xmlui-org/mcpsvr/xmluimcp/mcpsvr"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer represents an XMLUI MCP server instance
type MCPServer struct {
	config         common.ServerConfig
	mcpServer      *server.MCPServer
	sessionManager *SessionManager
	prompts        []mcp.Prompt
	tools          []mcp.Tool
	promptHandlers map[string]PromptHandler
}

// sendJSONResponse sends a JSON-encoded response to the HTTP client.
// If encoding fails, it logs the error and returns HTTP 500 to the client.
func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		mcpsvr.WriteDebugLog("Failed to marshal JSON: %v\n", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(jsonBytes); err != nil {
		mcpsvr.WriteDebugLog("Failed to write response: %v\n", err)
	}
}

// NewServer creates a new XMLUI MCP server with the given configuration
func NewServer(config common.ServerConfig) (*MCPServer, error) {
	// If XMLUIDir is not provided, automatically download and cache the repository
	if config.XMLUIDir == "" {
		mcpsvr.WriteDebugLog("No XMLUI directory specified, using cached repository...\n")
		cachedRepo, err := EnsureXMLUIRepo()
		if err != nil {
			return nil, fmt.Errorf("failed to ensure XMLUI repository: %w (you can specify a local XMLUI directory as an argument)", err)
		}
		config.XMLUIDir = cachedRepo
		mcpsvr.WriteDebugLog("Using cached XMLUI repository at: %s\n", config.XMLUIDir)
	}

	// Set defaults
	if config.Port == "" {
		config.Port = "8080"
	}
	if config.AnalyticsFile == "" {
		config.AnalyticsFile = filepath.Join(getCurrentDir(), "xmlui-mcp-analytics.json")
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("XMLUI", "0.1.0",
		server.WithPromptCapabilities(true),
	)

	// Initialize analytics
	mcpsvr.InitializeAnalytics(config.AnalyticsFile)

	// Create session manager
	sessionManager := &SessionManager{
		sessions: make(map[string]*SessionContext),
	}

	// Create XMLUI server instance
	xmluiServer := &MCPServer{
		config:         config,
		mcpServer:      mcpServer,
		sessionManager: sessionManager,
		prompts:        []mcp.Prompt{},
		tools:          []mcp.Tool{},
		promptHandlers: make(map[string]PromptHandler),
	}

	// Setup all tools and prompts
	if err := xmluiServer.setupTools(); err != nil {
		return nil, fmt.Errorf("failed to setup tools: %w", err)
	}

	if err := xmluiServer.setupPrompts(); err != nil {
		return nil, fmt.Errorf("failed to setup prompts: %w", err)
	}

	// Auto-inject XMLUI rules into default session
	_, err := sessionManager.InjectPrompt("default", "xmlui_rules", xmluiServer.promptHandlers)
	if err != nil {
		mcpsvr.WriteDebugLog("Failed to auto-inject xmlui_rules: %v\n", err)
	} else {
		mcpsvr.WriteDebugLog("Auto-injected xmlui_rules into default session\n")
	}

	return xmluiServer, nil
}

// setupTools registers all XMLUI tools with the MCP server
func (s *MCPServer) setupTools() error {
	// Build example roots from configuration
	dirCount := len(s.config.ExampleDirs)
	exampleRoots := make([]string, 0, dirCount)
	if s.config.ExampleRoot != "" && dirCount > 0 {
		for _, d := range s.config.ExampleDirs {
			trimmed := strings.TrimSpace(d)
			if trimmed != "" {
				exampleRoots = append(exampleRoots, filepath.Join(s.config.ExampleRoot, trimmed))
			}
		}
	}

	// List components tool
	listComponentsTool, listComponentsHandler := mcpsvr.NewListComponentsTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(listComponentsTool, mcpsvr.WithAnalytics("xmlui_list_components", listComponentsHandler))
	s.tools = append(s.tools, listComponentsTool)

	// Component docs tool
	componentDocsTool, componentDocsHandler := mcpsvr.NewComponentDocsTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(componentDocsTool, mcpsvr.WithAnalytics("xmlui_component_docs", componentDocsHandler))
	s.tools = append(s.tools, componentDocsTool)

	// Search docs tool
	searchDocsTool, searchDocsHandler := mcpsvr.NewSearchTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(searchDocsTool, mcpsvr.WithSearchAnalytics("xmlui_search", searchDocsHandler))
	s.tools = append(s.tools, searchDocsTool)

	// Read file tool
	readFileTool, readFileHandler := mcpsvr.NewReadFileTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(readFileTool, mcpsvr.WithAnalytics("xmlui_read_file", readFileHandler))
	s.tools = append(s.tools, readFileTool)

	// Examples tool
	examplesTool, examplesHandler := mcpsvr.NewExamplesTool(exampleRoots)
	s.mcpServer.AddTool(examplesTool, mcpsvr.WithSearchAnalytics("xmlui_examples", examplesHandler))
	s.tools = append(s.tools, examplesTool)

	// List howto tool
	listHowtoTool, listHowtoHandler := mcpsvr.NewListHowtoTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(listHowtoTool, mcpsvr.WithAnalytics("xmlui_list_howto", listHowtoHandler))
	s.tools = append(s.tools, listHowtoTool)

	// Search howto tool
	searchHowtoTool, searchHowtoHandler := mcpsvr.NewSearchHowtoTool(s.config.XMLUIDir)
	s.mcpServer.AddTool(searchHowtoTool, mcpsvr.WithSearchAnalytics("xmlui_search_howto", searchHowtoHandler))
	s.tools = append(s.tools, searchHowtoTool)

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

		if name := mcpsvr.RequestArgument(request, "prompt_name"); name != "" {
			promptName = name
		}
		if id := mcpsvr.RequestArgument(request, "session_id"); id != "" {
			sessionID = id
		}

		// Use the session manager to inject
		response, err := s.sessionManager.InjectPrompt(sessionID, promptName, s.promptHandlers)
		if err != nil {
			return mcp.NewToolResultError("Error injecting prompt: " + err.Error()), nil
		}

		if response.Success {
			return mcp.NewToolResultText(fmt.Sprintf("✅ Successfully injected '%s' prompt into session '%s'. The guidelines are now active in your context.", promptName, sessionID)), nil
		}

		return mcp.NewToolResultError("❌ Failed to inject prompt: " + response.Message), nil
	}

	s.mcpServer.AddTool(injectPromptTool, mcpsvr.WithAnalytics("xmlui_inject_prompt", injectPromptHandler))
	s.tools = append(s.tools, injectPromptTool)

	// Add prompt listing tool
	listPromptsTool := mcp.NewTool("xmlui_list_prompts",
		mcp.WithDescription("Lists all available prompts that can be injected into session context"),
	)

	listPromptsHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var out strings.Builder
		out.WriteString("Available prompts:\n\n")

		for _, prompt := range s.prompts {
			out.WriteString(fmt.Sprintf("- **%s**: %s\n", prompt.Name, prompt.Description))
		}

		out.WriteString("\nUse xmlui_get_prompt to view content or xmlui_inject_prompt to inject into context.")
		return mcp.NewToolResultText(out.String()), nil
	}

	s.mcpServer.AddTool(listPromptsTool, mcpsvr.WithAnalytics("xmlui_list_prompts", listPromptsHandler))
	s.tools = append(s.tools, listPromptsTool)

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
		promptName := mcpsvr.RequestArgument(request, "prompt_name")

		if promptName == "" {
			return mcp.NewToolResultError("prompt_name parameter is required"), nil
		}

		// Find the prompt
		var foundPrompt *mcp.Prompt
		for _, prompt := range s.prompts {
			if prompt.Name == promptName {
				foundPrompt = &prompt
				break
			}
		}

		if foundPrompt == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", promptName)), nil
		}

		// Get the prompt handler
		handler, exists := s.promptHandlers[promptName]
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

	s.mcpServer.AddTool(getPromptTool, mcpsvr.WithAnalytics("xmlui_get_prompt", getPromptHandler))
	s.tools = append(s.tools, getPromptTool)

	// Add session context retrieval tool
	getSessionContextTool := mcp.NewTool("xmlui_get_session_context",
		mcp.WithDescription("Retrieves the current session context including any injected prompts"),
	)

	getSessionContextTool.InputSchema = mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session ID (optional, defaults to 'default')",
				"default":     "default",
			},
		},
		Required: []string{},
	}

	getSessionContextHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract session ID
		sessionID := "default" // default
		if id := mcpsvr.RequestArgument(request, "session_id"); id != "" {
			sessionID = id
		}

		session := s.sessionManager.GetOrCreateSession(sessionID)

		// Format the output
		var out strings.Builder
		out.WriteString(fmt.Sprintf("# Session Context: %s\n\n", session.ID))
		out.WriteString(fmt.Sprintf("**Last Activity:** %s\n\n", session.LastActivity.Format(time.RFC3339)))
		out.WriteString(fmt.Sprintf("**Injected Prompts:** %s\n\n", strings.Join(session.InjectedPrompts, ", ")))

		if len(session.Context) > 0 {
			out.WriteString("**Context Content:**\n\n")
			for i, message := range session.Context {
				out.WriteString(fmt.Sprintf("### Message %d\n\n", i+1))
				switch content := message.Content.(type) {
				case *mcp.TextContent:
					out.WriteString(content.Text)
					out.WriteString("\n\n")
				case mcp.TextContent:
					out.WriteString(content.Text)
					out.WriteString("\n\n")
				}
			}
		} else {
			out.WriteString("**Context Content:** No content in session context.\n\n")
		}

		return mcp.NewToolResultText(out.String()), nil
	}

	s.mcpServer.AddTool(getSessionContextTool, mcpsvr.WithAnalytics("xmlui_get_session_context", getSessionContextHandler))
	s.tools = append(s.tools, getSessionContextTool)

	return nil
}

// setupPrompts registers all XMLUI prompts with the MCP server
func (s *MCPServer) setupPrompts() error {
	// Define the xmlui_rules prompt handler
	xmluiRulesHandler := func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult(
			"XMLUI Development Rules and Guidelines",
			[]mcp.PromptMessage{
				mcp.NewPromptMessage(
					mcp.RoleUser,
					mcp.NewTextContent(`You are assisting with XMLUI development. Follow these essential rules:

1 don't write any code without my permission, always preview proposed changes, discuss, and only proceed with approval

2 don't add any xmlui styling, let the theme and layout engine do its job

3 proceed in small increments, write the absolute minimum amount of xmlui markup necessary and no script if possible

4 do not invent any xmlui syntax, only use constructs for which you can find examples in the docs and sample apps, and always cite your sources.

5 never touch the dom, we only work within xmlui abstractions inside the <App> realm, with help from vars and functions defined on the window variable in index.html

6 keep complex functions and expressions out of xmlui, then can live in index.html or (if scoping requires) in code-behind

7 use this xmlui-mcp server to list and show component docs but also search xmlui source, docs, examples, and howto articles

8 always do the simplest thing possible

9 use a neutral tone, do not say "Perfect!" etc, in fact never use exclamation marks at all

10 when creating examples for live playgrounds, observe the conventions for ---app, ---comp and --api

11 VStack is the default, don't use it unless necessary

12 prioritize XMLUI tools, especially xmlui_list_howto and xmlui_search_howto, and always cite the urls of found articles

13 prioritize xmlui-pg examples in .md files under src/components

These rules ensure clean, maintainable XMLUI applications that follow best practices.`),
				),
			},
		), nil
	}

	// Create the xmlui_rules prompt
	xmluiRulesPrompt := mcp.NewPrompt("xmlui_rules",
		mcp.WithPromptDescription("Essential rules and guidelines for XMLUI development"))

	// Store in our lists for API access
	s.prompts = append(s.prompts, xmluiRulesPrompt)
	s.promptHandlers["xmlui_rules"] = xmluiRulesHandler

	// Register with MCP server
	s.mcpServer.AddPrompt(xmluiRulesPrompt, xmluiRulesHandler)

	return nil
}

// ServeStdio starts the server in stdio mode with graceful shutdown
func (s *MCPServer) ServeStdio() error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.ServeStdio(s.mcpServer)
	}()

	// Wait for either server error or signal
	select {
	case err := <-serverDone:
		// Server finished (normally or with error)
		if err != nil {
			mcpsvr.WriteDebugLog("Server error: %v\n", err)
			return err
		}
		return nil
	case <-sigChan:
		mcpsvr.WriteDebugLog("Received shutdown signal, initiating graceful shutdown\n")

		// Wait for server to shutdown gracefully with timeout
		select {
		case err := <-serverDone:
			if err != nil {
				mcpsvr.WriteDebugLog("Server shutdown with error: %v\n", err)
				return err
			}
			mcpsvr.WriteDebugLog("Server shutdown complete\n")
			return nil

		case <-time.After(5 * time.Second):
			mcpsvr.WriteDebugLog("Server shutdown timeout, forcing exit\n")
			return fmt.Errorf("server shutdown timeout")
		}
	}
}

// ServeHTTP starts the server in HTTP mode
func (s *MCPServer) ServeHTTP() error {
	sseServer := server.NewSSEServer(s.mcpServer)

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
		for _, tool := range s.tools {
			toolList = append(toolList, map[string]string{
				"name":        tool.Name,
				"description": tool.Description,
			})
		}

		sendJSONResponse(w, toolList)
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
		for _, prompt := range s.prompts {
			promptInfoList = append(promptInfoList, PromptInfo{
				Name:        prompt.Name,
				Description: prompt.Description,
			})
		}

		sendJSONResponse(w, promptInfoList)
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
		for _, prompt := range s.prompts {
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
		handler, exists := s.promptHandlers[promptName]
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

		sendJSONResponse(w, promptContent)
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

		session := s.sessionManager.GetOrCreateSession(sessionID)
		sendJSONResponse(w, session)
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

		response, err := s.sessionManager.InjectPrompt(req.SessionID, req.PromptName, s.promptHandlers)
		if err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		sendJSONResponse(w, response)
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

		summary := mcpsvr.GetAnalyticsSummary()
		sendJSONResponse(w, summary)
	})

	addr := ":" + s.config.Port
	mcpsvr.WriteDebugLog("Starting HTTP mcpsvr on port %s\n", s.config.Port)
	mcpsvr.WriteDebugLog("SSE endpoint: http://localhost%s/sse\n", addr)
	mcpsvr.WriteDebugLog("Message endpoint: http://localhost%s/message\n", addr)
	mcpsvr.WriteDebugLog("Tools endpoint: http://localhost%s/tools\n", addr)
	mcpsvr.WriteDebugLog("Prompts list endpoint: http://localhost%s/prompts\n", addr)
	mcpsvr.WriteDebugLog("Specific prompt endpoint: http://localhost%s/prompts/{name}\n", addr)
	mcpsvr.WriteDebugLog("Session context endpoint: http://localhost%s/session/{id}\n", addr)
	mcpsvr.WriteDebugLog("Inject prompt endpoint: http://localhost%s/session/context\n", addr)
	mcpsvr.WriteDebugLog("Analytics summary endpoint: http://localhost%s/analytics/summary\n", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		mcpsvr.WriteDebugLog("HTTP mcpsvr error: %v\n", err)
		return err
	}

	return nil
}

// GetTools returns the list of available tools
func (s *MCPServer) GetTools() []mcp.Tool {
	return s.tools
}

// GetPrompts returns the list of available prompts
func (s *MCPServer) GetPrompts() []mcp.Prompt {
	return s.prompts
}

// PrintStartupInfo prints server startup information as JSON to stderr
func (s *MCPServer) PrintStartupInfo() {
	//printStartupInfo(s.prompts, s.tools, s.promptHandlers["xmlui_rules"])
}

// GetSessionManager returns the session manager
func (s *MCPServer) GetSessionManager() *SessionManager {
	return s.sessionManager
}
