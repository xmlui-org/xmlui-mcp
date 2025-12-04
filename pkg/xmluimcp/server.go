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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpserver "xmlui-mcp/server"
)

// ServerConfig holds configuration for the XMLUI MCP server
type ServerConfig struct {
	ExampleDirs  []string // Optional: directories for examples
	HTTPMode     bool     // Whether to run in HTTP mode
	Port         string   // Port for HTTP mode (default: "8080")
	XMLUIVersion string   // Specific XMLUI version to use (e.g. "0.11.4")
}

// MCPServer represents an XMLUI MCP server instance
type MCPServer struct {
	config         ServerConfig
	xmluiDir       string // Path to cached XMLUI repository (set automatically)
	mcpServer      *server.MCPServer
	sessionManager *SessionManager
	prompts        []mcp.Prompt
	tools          []mcp.Tool
	promptHandlers map[string]PromptHandler
}

// NewServer creates a new XMLUI MCP server with the given configuration
func NewServer(config ServerConfig) (*MCPServer, error) {
	// Set up analytics file path first (needed for debug log path)
	// Put analytics file in cache directory for consistency
	cacheDir, err := GetCacheDir()
	var analyticsFile string
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to get cache directory for analytics file: %v\n", err)
		// Fallback to current directory if cache directory unavailable
		analyticsFile = filepath.Join(getCurrentDir(), "xmlui-mcp-analytics.json")
	} else {
		analyticsFile = filepath.Join(cacheDir, "xmlui-mcp-analytics.json")
	}

	// Set debug log path early, before any logging happens
	// This ensures all logs go to the cache directory, not the current working directory
	logPath := filepath.Join(filepath.Dir(analyticsFile), "xmlui-mcp-server.log")
	mcpserver.SetDebugLogPath(logPath)

	// Always download and cache the repository
	mcpserver.WriteDebugLog("Ensuring cached XMLUI repository is available...\n")
	cachedRepo, err := EnsureXMLUIRepo(config.XMLUIVersion)
	if err != nil {
		mcpserver.WriteDebugLog("ERROR: Failed to ensure XMLUI repository: %v\n", err)
		return nil, fmt.Errorf("Failed to ensure the presence of XMLUI repository: %w\nFor more information, check the logs at: %s", err, logPath)
	}
	mcpserver.WriteDebugLog("Using cached XMLUI repository at: %s\n", cachedRepo)

	// Set defaults
	if config.Port == "" {
		config.Port = "8080"
	}
	// AnalyticsFile is already set above

	// Create MCP server
	mcpServer := server.NewMCPServer("XMLUI", "0.1.0",
		server.WithPromptCapabilities(true),
	)

	// Initialize analytics
	mcpserver.InitializeAnalytics(analyticsFile)

	// Create session manager
	sessionManager := &SessionManager{
		sessions: make(map[string]*SessionContext),
	}

	// Create XMLUI server instance
	xmluiServer := &MCPServer{
		config:         config,
		xmluiDir:       cachedRepo,
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
	_, err = sessionManager.InjectPrompt("default", "xmlui_rules", xmluiServer.promptHandlers)
	if err != nil {
		mcpserver.WriteDebugLog("Failed to auto-inject xmlui_rules: %v\n", err)
	} else {
		mcpserver.WriteDebugLog("Auto-injected xmlui_rules into default session\n")
	}

	return xmluiServer, nil
}

// setupTools registers all XMLUI tools with the MCP server
func (s *MCPServer) setupTools() error {
	// Build example roots from configuration
	exampleRoots := []string{}
	for _, d := range s.config.ExampleDirs {
		trimmed := strings.TrimSpace(d)
		if trimmed != "" {
			exampleRoots = append(exampleRoots, trimmed)
		}
	}

	// List components tool
	listComponentsTool, listComponentsHandler := mcpserver.NewListComponentsTool(s.xmluiDir)
	s.mcpServer.AddTool(listComponentsTool, mcpserver.WithAnalytics("xmlui_list_components", listComponentsHandler))
	s.tools = append(s.tools, listComponentsTool)

	// Component docs tool
	componentDocsTool, componentDocsHandler := mcpserver.NewComponentDocsTool(s.xmluiDir)
	s.mcpServer.AddTool(componentDocsTool, mcpserver.WithAnalytics("xmlui_component_docs", componentDocsHandler))
	s.tools = append(s.tools, componentDocsTool)

	// Search docs tool
	searchDocsTool, searchDocsHandler := mcpserver.NewSearchTool(s.xmluiDir, exampleRoots)
	s.mcpServer.AddTool(searchDocsTool, mcpserver.WithSearchAnalytics("xmlui_search", searchDocsHandler))
	s.tools = append(s.tools, searchDocsTool)

	// Read file tool
	readFileTool, readFileHandler := mcpserver.NewReadFileTool(s.xmluiDir)
	s.mcpServer.AddTool(readFileTool, mcpserver.WithAnalytics("xmlui_read_file", readFileHandler))
	s.tools = append(s.tools, readFileTool)

	// Examples tool
	examplesTool, examplesHandler := mcpserver.NewExamplesTool(exampleRoots)
	s.mcpServer.AddTool(examplesTool, mcpserver.WithSearchAnalytics("xmlui_examples", examplesHandler))
	s.tools = append(s.tools, examplesTool)

	// List howto tool
	listHowtoTool, listHowtoHandler := mcpserver.NewListHowtoTool(s.xmluiDir)
	s.mcpServer.AddTool(listHowtoTool, mcpserver.WithAnalytics("xmlui_list_howto", listHowtoHandler))
	s.tools = append(s.tools, listHowtoTool)

	// Search howto tool
	searchHowtoTool, searchHowtoHandler := mcpserver.NewSearchHowtoTool(s.xmluiDir)
	s.mcpServer.AddTool(searchHowtoTool, mcpserver.WithSearchAnalytics("xmlui_search_howto", searchHowtoHandler))
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

		if request.Params.Arguments != nil {
			if name, ok := request.Params.Arguments["prompt_name"].(string); ok && name != "" {
				promptName = name
			}
			if id, ok := request.Params.Arguments["session_id"].(string); ok && id != "" {
				sessionID = id
			}
		}

		// Use the session manager to inject
		response, err := s.sessionManager.InjectPrompt(sessionID, promptName, s.promptHandlers)
		if err != nil {
			return mcp.NewToolResultError("Error injecting prompt: " + err.Error()), nil
		}

		if response.Success {
			return mcp.NewToolResultText(fmt.Sprintf("✅ Successfully injected '%s' prompt into session '%s'. The guidelines are now active in your context.", promptName, sessionID)), nil
		} else {
			return mcp.NewToolResultError("❌ Failed to inject prompt: " + response.Message), nil
		}
	}

	s.mcpServer.AddTool(injectPromptTool, mcpserver.WithAnalytics("xmlui_inject_prompt", injectPromptHandler))
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

	s.mcpServer.AddTool(listPromptsTool, mcpserver.WithAnalytics("xmlui_list_prompts", listPromptsHandler))
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

	s.mcpServer.AddTool(getPromptTool, mcpserver.WithAnalytics("xmlui_get_prompt", getPromptHandler))
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
		if request.Params.Arguments != nil {
			if id, ok := request.Params.Arguments["session_id"].(string); ok && id != "" {
				sessionID = id
			}
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

	s.mcpServer.AddTool(getSessionContextTool, mcpserver.WithAnalytics("xmlui_get_session_context", getSessionContextHandler))
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
	fmt.Fprintf(os.Stderr, "Listening for messages on standard input...\n")

	// Wait for either server error or signal
	select {
	case err := <-serverDone:
		// Server finished (normally or with error)
		if err != nil {
			mcpserver.WriteDebugLog("Server error: %v\n", err)
			return err
		}
		return nil
	case <-sigChan:
		mcpserver.WriteDebugLog("Received shutdown signal, initiating graceful shutdown\n")

		// Wait for server to shutdown gracefully with timeout
		select {
		case err := <-serverDone:
			if err != nil {
				mcpserver.WriteDebugLog("Server shutdown with error: %v\n", err)
				return err
			} else {
				mcpserver.WriteDebugLog("Server shutdown complete\n")
				return nil
			}
		case <-time.After(5 * time.Second):
			mcpserver.WriteDebugLog("Server shutdown timeout, forcing exit\n")
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
		for _, prompt := range s.prompts {
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

		session := s.sessionManager.GetOrCreateSession(sessionID)
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

		response, err := s.sessionManager.InjectPrompt(req.SessionID, req.PromptName, s.promptHandlers)
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

	addr := ":" + s.config.Port
	mcpserver.WriteDebugLog("Starting HTTP server on port %s\n", s.config.Port)
	fmt.Fprintf(os.Stderr, "Server listening on http://localhost%s...\n", addr)
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
	printStartupInfo(s.prompts, s.tools, s.promptHandlers["xmlui_rules"])
}

// GetSessionManager returns the session manager
func (s *MCPServer) GetSessionManager() *SessionManager {
	return s.sessionManager
}
