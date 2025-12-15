package server

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Analytics structures for tracking agent usage
type ToolInvocation struct {
	Type       string                 `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Success    bool                   `json:"success"`
	ResultSize int                    `json:"result_size_chars"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
}

type SearchQuery struct {
	Type        string    `json:"type"`
	Timestamp   time.Time `json:"timestamp"`
	ToolName    string    `json:"tool_name"`
	Query       string    `json:"query"`
	ResultCount int       `json:"result_count"`
	Success     bool      `json:"success"`
	SearchPaths []string  `json:"search_paths,omitempty"`
	FoundURLs   []string  `json:"found_urls,omitempty"`
}

type AnalyticsData struct {
	ToolInvocations []ToolInvocation `json:"tool_invocations"`
	SearchQueries   []SearchQuery    `json:"search_queries"`
}

type Analytics struct {
	mu      sync.RWMutex
	data    AnalyticsData
	logFile string
}

func InitializeAnalytics(logFile string) {
	// Initialize analytics storage
	globalAnalytics = newAnalytics(logFile)
	WriteDebugLog("[DEBUG] Initializing analytics with log file: %s\n", logFile)
	WriteDebugLog("[DEBUG] Analytics initialized, globalAnalytics is nil: %v\n", globalAnalytics == nil)
}

func (a *Analytics) GetSummary() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Tool usage frequency
	toolCounts := make(map[string]int)
	successRates := make(map[string]float64)
	toolSuccesses := make(map[string]int)
	avgResultSizes := make(map[string]int)
	toolResultSizes := make(map[string][]int)

	for _, inv := range a.data.ToolInvocations {
		toolCounts[inv.ToolName]++
		if inv.Success {
			toolSuccesses[inv.ToolName]++
		}
		toolResultSizes[inv.ToolName] = append(toolResultSizes[inv.ToolName], inv.ResultSize)
	}

	for tool, count := range toolCounts {
		successRates[tool] = float64(toolSuccesses[tool]) / float64(count) * 100

		// Calculate average result size
		sizes := toolResultSizes[tool]
		if len(sizes) > 0 {
			var total int
			for _, s := range sizes {
				total += s
			}
			avgResultSizes[tool] = total / len(sizes)
		}
	}

	// Search query analysis
	searchTerms := make(map[string]int)
	searchSuccessRate := 0.0
	if len(a.data.SearchQueries) > 0 {
		successCount := 0
		for _, sq := range a.data.SearchQueries {
			searchTerms[sq.Query]++
			if sq.Success {
				successCount++
			}
		}
		searchSuccessRate = float64(successCount) / float64(len(a.data.SearchQueries)) * 100
	}

	return map[string]interface{}{
		"total_tool_invocations": len(a.data.ToolInvocations),
		"total_search_queries":   len(a.data.SearchQueries),
		"tool_usage_counts":      toolCounts,
		"tool_success_rates":     successRates,
		"tool_avg_result_sizes":  avgResultSizes,
		"search_success_rate":    searchSuccessRate,
		"popular_search_terms":   searchTerms,
	}
}

// Wrapper function to add analytics to any tool handler
func WithAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// DEBUG: Log entry into withAnalytics wrapper
		WriteDebugLog("[DEBUG] withAnalytics ENTRY: tool=%s\n", toolName)

		// DEBUG: Log before calling original handler
		WriteDebugLog("[DEBUG] withAnalytics BEFORE_HANDLER: tool=%s\n", toolName)

		// Call the original handler
		result, err := handler(ctx, req)

		// DEBUG: Log after calling original handler
		WriteDebugLog("[DEBUG] withAnalytics AFTER_HANDLER: tool=%s, err=%v, result_nil=%v\n", toolName, err, result == nil)

		// Calculate metrics
		success := err == nil && result != nil
		resultSize := 0
		errorMsg := ""

		if result != nil {
			// Estimate result size based on content
			for _, content := range result.Content {
				switch c := content.(type) {
				case *mcp.TextContent:
					resultSize += len(c.Text)
				case mcp.TextContent:
					resultSize += len(c.Text)
				}
			}
		}

		if err != nil {
			errorMsg = err.Error()
		} else if result != nil && !result.IsError {
			// Check if result indicates an error
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					if len(textContent.Text) > 0 && textContent.Text[0:1] == "❌" {
						success = false
						errorMsg = "Tool returned error result"
						break
					}
				}
			}
		}

		// DEBUG: Log before calling LogTool
		WriteDebugLog("[DEBUG] withAnalytics BEFORE_LOGGING: tool=%s, success=%v, resultSize=%d\n", toolName, success, resultSize)

		// Log the invocation
		logTool(toolName, req.Params.Arguments, success, resultSize, errorMsg)

		// DEBUG: Log after calling LogTool
		WriteDebugLog("[DEBUG] withAnalytics AFTER_LOGGING: tool=%s\n", toolName)

		return result, err
	}
}

// Special wrapper for search tools to capture additional search-specific metrics
func WithSearchAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract search query
		query := ""
		if req.Params.Arguments != nil {
			if q, ok := req.Params.Arguments["query"].(string); ok {
				query = q
			}
		}

		// Call the original handler
		result, err := handler(ctx, req)

		// Calculate metrics
		toolSuccess := err == nil && result != nil
		resultSize := 0
		errorMsg := ""
		resultCount := 0

		if result != nil {
			// Estimate result size and count matches
			for _, content := range result.Content {
				switch c := content.(type) {
				case *mcp.TextContent:
					text := c.Text
					resultSize += len(text)
					// Rough estimate of result count based on line breaks
					if text != "No matches found." && text != "No examples found." {
						resultCount = len(splitLines(text))
					}
				case mcp.TextContent:
					text := c.Text
					resultSize += len(text)
					if text != "No matches found." && text != "No examples found." {
						resultCount = len(splitLines(text))
					}
				}
			}
		}

		if err != nil {
			errorMsg = err.Error()
		}

		// For search tools, success should reflect whether results were found
		searchSuccess := toolSuccess && resultCount > 0

		// Log both general tool usage and specific search metrics
		logTool(toolName, req.Params.Arguments, toolSuccess, resultSize, errorMsg)

		// Log search-specific data with search-specific success metric
		searchPaths := getSearchPaths(toolName)
		foundURLs := extractFoundURLs(result)
		logSearch(toolName, query, resultCount, searchSuccess, searchPaths, foundURLs)

		return result, err
	}
}

func newAnalytics(logFile string) *Analytics {
	a := &Analytics{
		data: AnalyticsData{
			ToolInvocations: make([]ToolInvocation, 0),
			SearchQueries:   make([]SearchQuery, 0),
		},
		logFile: logFile,
	}

	// Load existing data if available
	a.loadData()

	return a
}

func (a *Analytics) loadData() {
	if _, err := os.Stat(a.logFile); os.IsNotExist(err) {
		return
	}

	content, err := os.ReadFile(a.logFile)
	if err != nil {
		WriteDebugLog("Failed to load analytics data: %v\n", err)
		return
	}

	// Parse JSONL format - one JSON object per line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// First, try to determine the type from the JSON
		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &typeCheck); err != nil {
			WriteDebugLog("Failed to parse type from analytics line: %s\n", line)
			continue
		}

		// Parse based on type
		switch typeCheck.Type {
		case "tool_invocation":
			var invocation ToolInvocation
			if err := json.Unmarshal([]byte(line), &invocation); err == nil {
				a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)
			} else {
				WriteDebugLog("Failed to parse tool_invocation: %s\n", line)
			}
		case "search_query":
			var searchQuery SearchQuery
			if err := json.Unmarshal([]byte(line), &searchQuery); err == nil {
				a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)
			} else {
				WriteDebugLog("Failed to parse search_query: %s\n", line)
			}
		default:
			// Skip session_activity records
			continue
		}
	}
}

func (a *Analytics) writeLine(data interface{}) {
	WriteDebugLog("[DEBUG] writeLine ENTRY: file=%s\n", a.logFile)

	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_MARSHAL: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine MARSHALED: %s\n", string(jsonData))

	// Open file for appending
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_OPEN: %v\n", err)
		return
	}
	defer file.Close()

	WriteDebugLog("[DEBUG] writeLine FILE_OPENED: %s\n", a.logFile)

	// Write the JSON line
	bytesWritten, err := file.Write(jsonData)
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_WRITE: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine WROTE_JSON: %d bytes\n", bytesWritten)

	// Write newline
	newlineBytes, err := file.Write([]byte("\n"))
	if err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_NEWLINE: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine WROTE_NEWLINE: %d bytes\n", newlineBytes)

	// Sync to ensure it's written immediately
	if err := file.Sync(); err != nil {
		WriteDebugLog("[DEBUG] writeLine FAILED_SYNC: %v\n", err)
		return
	}

	WriteDebugLog("[DEBUG] writeLine SUCCESS: wrote %d total bytes\n", bytesWritten+newlineBytes)
}

func (a *Analytics) logToolInvocation(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	WriteDebugLog("[DEBUG] LogToolInvocation ENTRY: tool=%s\n", toolName)

	a.mu.Lock()
	defer a.mu.Unlock()

	invocation := ToolInvocation{
		Type:       "tool_invocation",
		Timestamp:  time.Now(),
		ToolName:   toolName,
		Arguments:  args,
		Success:    success,
		ResultSize: resultSize,
		ErrorMsg:   errorMsg,
	}

	WriteDebugLog("[DEBUG] LogToolInvocation BEFORE_APPEND: tool=%s, current_count=%d\n", toolName, len(a.data.ToolInvocations))

	a.data.ToolInvocations = append(a.data.ToolInvocations, invocation)

	WriteDebugLog("[DEBUG] LogToolInvocation AFTER_APPEND: tool=%s, new_count=%d\n", toolName, len(a.data.ToolInvocations))

	// Write debug info to server.log file
	WriteDebugLog("[DEBUG] LogToolInvocation: tool=%s, success=%v, resultSize=%d\n",
		toolName, success, resultSize)

	// Save immediately for stdio mode (each call is a separate process)
	WriteDebugLog("[DEBUG] LogToolInvocation BEFORE_WRITELINE: calling writeLine\n")
	a.writeLine(invocation)
	WriteDebugLog("[DEBUG] LogToolInvocation AFTER_WRITELINE: writeLine completed\n")
}

func (a *Analytics) logSearchQuery(toolName string, query string, resultCount int, success bool, searchPaths []string, foundURLs []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Debug to server.log instead of analytics file
	WriteDebugLog("[DEBUG] LogSearchQuery ENTRY: tool=%s, query=%s\n", toolName, query)

	searchQuery := SearchQuery{
		Type:        "search_query",
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Query:       query,
		ResultCount: resultCount,
		Success:     success,
		SearchPaths: searchPaths,
		FoundURLs:   foundURLs,
	}

	a.data.SearchQueries = append(a.data.SearchQueries, searchQuery)

	// Debug before writeLine
	WriteDebugLog("[DEBUG] LogSearchQuery BEFORE_WRITELINE: calling writeLine\n")

	// Save immediately for stdio mode
	a.writeLine(searchQuery)

	// Debug after writeLine
	WriteDebugLog("[DEBUG] LogSearchQuery AFTER_WRITELINE: writeLine completed\n")
}

// Global analytics instance
var globalAnalytics *Analytics

func logTool(toolName string, args map[string]interface{}, success bool, resultSize int, errorMsg string) {
	WriteDebugLog("[DEBUG] LogTool ENTRY: tool=%s, globalAnalytics_nil=%v\n", toolName, globalAnalytics == nil)

	if globalAnalytics != nil {
		WriteDebugLog("[DEBUG] LogTool CALLING_LogToolInvocation: tool=%s\n", toolName)
		globalAnalytics.logToolInvocation(toolName, args, success, resultSize, errorMsg)
		WriteDebugLog("[DEBUG] LogTool AFTER_LogToolInvocation: tool=%s\n", toolName)
	} else {
		WriteDebugLog("[DEBUG] LogTool SKIPPED: globalAnalytics is nil for tool=%s\n", toolName)
	}
}

func logSearch(toolName string, query string, resultCount int, success bool, searchPaths []string, foundURLs []string) {
	if globalAnalytics != nil {
		WriteDebugLog("[DEBUG] LogSearch ENTRY: tool=%s, query=%s\n", toolName, query)
		globalAnalytics.logSearchQuery(toolName, query, resultCount, success, searchPaths, foundURLs)
		WriteDebugLog("[DEBUG] LogSearch AFTER_LogSearchQuery: tool=%s\n", toolName)
	}
}

func GetAnalyticsSummary() map[string]interface{} {
	if globalAnalytics != nil {
		return globalAnalytics.GetSummary()
	}
	return map[string]interface{}{}
}

// Helper function to split text into lines for counting results
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}

	lines := []string{}
	current := ""

	for _, char := range text {
		if char == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

// Helper to get search paths for different tools
func getSearchPaths(toolName string) []string {
	switch toolName {
	case "xmlui_search":
		return []string{
			"docs/content/components",
			"docs/content/pages",
			"docs/public/pages",
			"docs/src/components",
			"xmlui/src/components",
			"blog",
		}
	case "xmlui_examples":
		return []string{"example_roots"} // This would be populated from actual example roots
	case "xmlui_search_howto":
		return []string{"docs/content/pages/howto.md", "docs/public/pages/howto.md"}
	default:
		return []string{}
	}
}

// extractFoundURLs parses the text result and returns a unique list of file-like URLs/paths
func extractFoundURLs(result *mcp.CallToolResult) []string {
	if result == nil {
		return []string{}
	}

	// Collect lines from text content
	lines := []string{}
	for _, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			lines = append(lines, splitLines(c.Text)...)
		case mcp.TextContent:
			lines = append(lines, splitLines(c.Text)...)
		}
	}

	// Extract before ':' as path for lines like "path:line: text" or "path: [filename match]"
	seen := map[string]struct{}{}
	out := []string{}
	for _, line := range lines {
		// Skip obvious non-matches
		if line == "" || line == "No matches found." || line == "No examples found." {
			continue
		}
		// Find first ':'
		idx := -1
		for i, ch := range line {
			if ch == ':' {
				idx = i
				break
			}
		}
		if idx <= 0 {
			continue
		}
		path := line[:idx]
		// Basic sanity: must contain a path separator and a file extension-ish dot
		if (containsRune(path, '/') || containsRune(path, '\\')) && containsRune(path, '.') {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				out = append(out, path)
			}
		}
	}
	return out
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
