package xmluimcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpsvr"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer represents an XMLUI MCP server instance
type MCPServer struct {
	config         *common.ServerConfig
	mcpServer      *server.MCPServer
	sessionManager *SessionManager
	prompts        []mcp.Prompt
	tools          []mcp.Tool
	promptHandlers map[string]PromptHandler
	logger         *slog.Logger
}

func (svr *MCPServer) Config() *common.ServerConfig {
	return svr.config
}

// sendJSONResponse sends a JSON-encoded response to the HTTP client.
// If encoding fails, it logs the error and returns HTTP 500 to the client.
func (svr *MCPServer) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		svr.logger.Debug("Failed to marshal JSON", "error", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(jsonBytes); err != nil {
		svr.logger.Debug("Failed to write response", "error", err)
	}
}

// NewServer creates a new XMLUI MCP server with the given configuration
func NewServer(config *common.Config) (svr *MCPServer) {
	svrCfg := config.Server
	// Set defaults
	if svrCfg.Port == "" {
		svrCfg.Port = "8080"
	}

	// Create MCP svrCfg
	mcpServer := server.NewMCPServer("XMLUI", "0.1.0",
		server.WithPromptCapabilities(true),
	)

	// Create XMLUI svrCfg instance
	return &MCPServer{
		config:         svrCfg,
		mcpServer:      mcpServer,
		sessionManager: NewSessionManager(),
		prompts:        []mcp.Prompt{},
		tools:          []mcp.Tool{},
		promptHandlers: make(map[string]PromptHandler),
		logger:         config.Logger,
	}
}

// Initialize performs initialization of an XMLUI MCP server that can generate errors
func (svr *MCPServer) Initialize() (err error) {
	svrCfg := svr.config
	logger := svr.logger

	// If XMLUIDir is not provided, automatically download and cache the repository
	if svrCfg.XMLUIDir == "" {
		var cachedRepo string

		logger.Debug("No XMLUI directory specified, using cached repository.")
		cachedRepo, err = EnsureXMLUIRepo(svr.logger)
		if err != nil {
			err = fmt.Errorf("failed to ensure XMLUI repository: %w (you can specify a local XMLUI directory as an argument)", err)
			goto end
		}
		svrCfg.XMLUIDir = cachedRepo
		logger.Debug("Using cached XMLUI repository", "cache_dir", svrCfg.XMLUIDir)
	}

	if svrCfg.AnalyticsFile == "" {
		var dir string
		dir, err = os.Getwd()
		if err != nil {
			err = fmt.Errorf(
				"cannot determine current working directory "+
					"(it may have been deleted, moved, or become "+
					" inaccessible; try running `pwd` and `ls -ld .` "+
					"in your shell): %w",
				err,
			)
			goto end
		}

		svrCfg.AnalyticsFile = filepath.Join(dir, "xmlui-mcp-analytics.json")
	}

	// Initialize analytics
	err = mcpsvr.InitializeAnalytics(svr.logger)
	if err != nil {
		err = fmt.Errorf("failed to initialize analytics: %w", err)
		goto end
	}

	// Setup all tools and prompts
	err = svr.setupTools()
	if err != nil {
		err = fmt.Errorf("failed to setup tools: %w", err)
		goto end
	}

	err = svr.setupPrompts()
	if err != nil {
		err = fmt.Errorf("failed to setup prompts: %w", err)
		goto end
	}

	// Auto-inject XMLUI rules into default session
	_, err = svr.sessionManager.InjectPrompt("default", "xmlui_rules", svr.promptHandlers)
	if err != nil {
		logger.Debug("Failed to auto-inject xmlui_rules", "error", err)
		goto end
	}

	logger.Info("Auto-injected xmlui_rules into default session")

end:
	return err
}

// setupTools registers all XMLUI tools with the MCP server
func (svr *MCPServer) setupTools() error {
	// Build example roots from configuration
	dirCount := len(svr.config.ExampleDirs)
	exampleRoots := make([]string, 0, dirCount)
	if svr.config.ExampleRoot != "" && dirCount > 0 {
		for _, d := range svr.config.ExampleDirs {
			trimmed := strings.TrimSpace(d)
			if trimmed != "" {
				exampleRoots = append(exampleRoots, filepath.Join(svr.config.ExampleRoot, trimmed))
			}
		}
	}

	// List components tool
	listComponentsTool, listComponentsHandler := mcpsvr.NewListComponentsTool(svr.config.XMLUIDir)
	svr.mcpServer.AddTool(listComponentsTool, mcpsvr.WithAnalytics("xmlui_list_components", listComponentsHandler, svr.logger))
	svr.tools = append(svr.tools, listComponentsTool)

	// Component docs tool
	componentDocsTool, componentDocsHandler := mcpsvr.NewComponentDocsTool(svr.config.XMLUIDir)
	svr.mcpServer.AddTool(componentDocsTool, mcpsvr.WithAnalytics("xmlui_component_docs", componentDocsHandler, svr.logger))
	svr.tools = append(svr.tools, componentDocsTool)

	// Search docs tool
	searchDocsTool, searchDocsHandler := mcpsvr.NewSearchTool(svr.config.XMLUIDir)
	svr.mcpServer.AddTool(searchDocsTool, mcpsvr.WithSearchAnalytics("xmlui_search", searchDocsHandler, svr.logger))
	svr.tools = append(svr.tools, searchDocsTool)

	// Read file tool
	readFileTool, readFileHandler := mcpsvr.NewReadFileTool(svr.config.XMLUIDir)
	svr.mcpServer.AddTool(readFileTool, mcpsvr.WithAnalytics("xmlui_read_file", readFileHandler, svr.logger))
	svr.tools = append(svr.tools, readFileTool)

	// Examples tool
	examplesTool, examplesHandler := mcpsvr.NewExamplesTool(exampleRoots, svr.logger)
	svr.mcpServer.AddTool(examplesTool, mcpsvr.WithSearchAnalytics("xmlui_examples", examplesHandler, svr.logger))
	svr.tools = append(svr.tools, examplesTool)

	// List howto tool
	listHowtoTool, listHowtoHandler := mcpsvr.NewListHowtoTool(svr.config.XMLUIDir, svr.logger)
	svr.mcpServer.AddTool(listHowtoTool, mcpsvr.WithAnalytics("xmlui_list_howto", listHowtoHandler, svr.logger))
	svr.tools = append(svr.tools, listHowtoTool)

	// Search howto tool
	searchHowtoTool, searchHowtoHandler := mcpsvr.NewSearchHowtoTool(svr.config.XMLUIDir)
	svr.mcpServer.AddTool(searchHowtoTool, mcpsvr.WithSearchAnalytics("xmlui_search_howto", searchHowtoHandler, svr.logger))
	svr.tools = append(svr.tools, searchHowtoTool)

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
		response, err := svr.sessionManager.InjectPrompt(sessionID, promptName, svr.promptHandlers)
		if err != nil {
			return mcp.NewToolResultError("Error injecting prompt: " + err.Error()), nil
		}

		if response.Success {
			return mcp.NewToolResultText(fmt.Sprintf("✅ Successfully injected '%s' prompt into session '%s'. The guidelines are now active in your context.", promptName, sessionID)), nil
		}

		return mcp.NewToolResultError("❌ Failed to inject prompt: " + response.Message), nil
	}

	svr.mcpServer.AddTool(injectPromptTool, mcpsvr.WithAnalytics("xmlui_inject_prompt", injectPromptHandler, svr.logger))
	svr.tools = append(svr.tools, injectPromptTool)

	// Add prompt listing tool
	listPromptsTool := mcp.NewTool("xmlui_list_prompts",
		mcp.WithDescription("Lists all available prompts that can be injected into session context"),
	)

	listPromptsHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var out strings.Builder
		out.WriteString("Available prompts:\n\n")

		for _, prompt := range svr.prompts {
			out.WriteString(fmt.Sprintf("- **%s**: %s\n", prompt.Name, prompt.Description))
		}

		out.WriteString("\nUse xmlui_get_prompt to view content or xmlui_inject_prompt to inject into context.")
		return mcp.NewToolResultText(out.String()), nil
	}

	svr.mcpServer.AddTool(listPromptsTool, mcpsvr.WithAnalytics("xmlui_list_prompts", listPromptsHandler, svr.logger))
	svr.tools = append(svr.tools, listPromptsTool)

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
		for _, prompt := range svr.prompts {
			if prompt.Name == promptName {
				foundPrompt = &prompt
				break
			}
		}

		if foundPrompt == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", promptName)), nil
		}

		// Get the prompt handler
		handler, exists := svr.promptHandlers[promptName]
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

	svr.mcpServer.AddTool(getPromptTool, mcpsvr.WithAnalytics("xmlui_get_prompt", getPromptHandler, svr.logger))
	svr.tools = append(svr.tools, getPromptTool)

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

		session := svr.sessionManager.GetOrCreateSession(sessionID)

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

	svr.mcpServer.AddTool(getSessionContextTool, mcpsvr.WithAnalytics("xmlui_get_session_context", getSessionContextHandler, svr.logger))
	svr.tools = append(svr.tools, getSessionContextTool)

	return nil
}

// setupPrompts registers all XMLUI prompts with the MCP server
func (svr *MCPServer) setupPrompts() error {
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
	svr.prompts = append(svr.prompts, xmluiRulesPrompt)
	svr.promptHandlers["xmlui_rules"] = xmluiRulesHandler

	// Register with MCP server
	svr.mcpServer.AddPrompt(xmluiRulesPrompt, xmluiRulesHandler)

	return nil
}

// ServeStdio starts the server in stdio mode with graceful shutdown
func (svr *MCPServer) ServeStdio() error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.ServeStdio(svr.mcpServer)
	}()

	// Wait for either server error or signal
	select {
	case err := <-serverDone:
		// Server finished (normally or with error)
		if err != nil {
			svr.logger.Error("Server error", "error", err)
			return err
		}
		return nil
	case <-sigChan:
		svr.logger.Info("Received shutdown signal, initiating graceful shutdown")

		// Wait for server to shutdown gracefully with timeout
		select {
		case err := <-serverDone:
			if err != nil {
				svr.logger.Error("Server shutdown with error", "error", err)
				return err
			}
			svr.logger.Info("Server shutdown complete")
			return nil

		case <-time.After(5 * time.Second):
			svr.logger.Warn("Server shutdown timeout, forcing exit")
			return fmt.Errorf("server shutdown timeout")
		}
	}
}

// ServeHTTP starts the server in HTTP mode
func (svr *MCPServer) ServeHTTP() error {
	sseServer := server.NewSSEServer(svr.mcpServer)

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
		for _, tool := range svr.tools {
			toolList = append(toolList, map[string]string{
				"name":        tool.Name,
				"description": tool.Description,
			})
		}

		svr.sendJSONResponse(w, toolList)
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
		for _, prompt := range svr.prompts {
			promptInfoList = append(promptInfoList, PromptInfo{
				Name:        prompt.Name,
				Description: prompt.Description,
			})
		}

		svr.sendJSONResponse(w, promptInfoList)
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
		for _, prompt := range svr.prompts {
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
		handler, exists := svr.promptHandlers[promptName]
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

		svr.sendJSONResponse(w, promptContent)
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

		session := svr.sessionManager.GetOrCreateSession(sessionID)
		svr.sendJSONResponse(w, session)
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

		response, err := svr.sessionManager.InjectPrompt(req.SessionID, req.PromptName, svr.promptHandlers)
		if err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		svr.sendJSONResponse(w, response)
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
		svr.sendJSONResponse(w, summary)
	})
	addr := ":" + svr.config.Port
	endpoint := func(path string) string {
		return fmt.Sprintf("http://localhost:%s/%s\n", addr, path)
	}
	logger := svr.logger
	logger.Info("Starting HTTP mcpsvr", "http_port", svr.config.Port)
	logger.Info("SSE endpoint", "endpoint", endpoint("sse"))
	logger.Info("Message endpoint", "endpoint", endpoint("message"))
	logger.Info("Tools endpoint", "endpoint", endpoint("tools"))
	logger.Info("Prompts list endpoint", "endpoint", endpoint("prompts"))
	logger.Info("Specific prompt endpoint", "endpoint", endpoint("prompts/{name}"))
	logger.Info("Session context endpoint", "endpoint", endpoint("session/{id}"))
	logger.Info("Inject prompt endpoint", "endpoint", endpoint("session/context"))
	logger.Info("Analytics summary endpoint", "endpoint", endpoint("analytics/summary"))

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Info("HTTP MCP Server error", "error", err)
		return err
	}

	return nil
}

// GetTools returns the list of available tools
func (svr *MCPServer) GetTools() []mcp.Tool {
	return svr.tools
}

// GetPrompts returns the list of available prompts
func (svr *MCPServer) GetPrompts() []mcp.Prompt {
	return svr.prompts
}

// PrintStartupInfo prints server startup information as JSON to stderr
func (svr *MCPServer) PrintStartupInfo() {
	printStartupInfo(svr.prompts, svr.tools, svr.promptHandlers["xmlui_rules"], svr.logger)
}

// GetSessionManager returns the session manager
func (svr *MCPServer) GetSessionManager() *SessionManager {
	return svr.sessionManager
}
