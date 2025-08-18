package server

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Global debug log file handle and mutex for thread-safe writing
var (
	debugLogFile  *os.File
	debugLogMutex sync.Mutex
)

// writeDebugLog writes debug messages to server.log in a thread-safe way
func writeDebugLog(format string, args ...interface{}) {
	debugLogMutex.Lock()
	defer debugLogMutex.Unlock()

	// Resolve target path for server.log
	targetPath := "server.log"
	if debugLogPath != "" {
		targetPath = debugLogPath
	}

	// Open file if not already open
	if debugLogFile == nil {
		var err error
		debugLogFile, err = os.OpenFile(targetPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Last resort: write to stderr
			fmt.Fprintf(os.Stderr, "Failed to open debug log file %s: %v\n", targetPath, err)
			return
		}
	}

	// Write the message
	fmt.Fprintf(debugLogFile, format, args...)

	// Flush immediately to ensure it's written
	debugLogFile.Sync()
}

// Wrapper function to add analytics to any tool handler
func WithAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// DEBUG: Log entry into withAnalytics wrapper
		writeDebugLog("[DEBUG] withAnalytics ENTRY: tool=%s\n", toolName)

		// Write to server.log
		writeDebugLog("[DEBUG] %s withAnalytics ENTRY: tool=%s\n", time.Now().Format("15:04:05.000"), toolName)

		// DEBUG: Log before calling original handler
		writeDebugLog("[DEBUG] withAnalytics BEFORE_HANDLER: tool=%s\n", toolName)
		writeDebugLog("[DEBUG] withAnalytics BEFORE_HANDLER: tool=%s\n", toolName)

		// Call the original handler
		result, err := handler(ctx, req)

		// DEBUG: Log after calling original handler
		writeDebugLog("[DEBUG] withAnalytics AFTER_HANDLER: tool=%s, err=%v, result_nil=%v\n", toolName, err, result == nil)
		writeDebugLog("[DEBUG] withAnalytics AFTER_HANDLER: tool=%s, err=%v, result_nil=%v\n", toolName, err, result == nil)

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
		writeDebugLog("[DEBUG] withAnalytics BEFORE_LOGGING: tool=%s, success=%v, resultSize=%d\n", toolName, success, resultSize)
		writeDebugLog("[DEBUG] withAnalytics BEFORE_LOGGING: tool=%s, success=%v, resultSize=%d\n", toolName, success, resultSize)

		// Log the invocation
		LogTool(toolName, req.Params.Arguments, success, resultSize, errorMsg)

		// DEBUG: Log after calling LogTool
		writeDebugLog("[DEBUG] withAnalytics AFTER_LOGGING: tool=%s\n", toolName)
		writeDebugLog("[DEBUG] withAnalytics AFTER_LOGGING: tool=%s\n", toolName)

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
		LogTool(toolName, req.Params.Arguments, toolSuccess, resultSize, errorMsg)

		// Log search-specific data with search-specific success metric
		searchPaths := getSearchPaths(toolName)
		foundURLs := extractFoundURLs(result)
		LogSearch(toolName, query, resultCount, searchSuccess, searchPaths, foundURLs)

		return result, err
	}
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
			"docs/public/pages",
			"docs/src/components",
			"xmlui/src/components",
		}
	case "xmlui_examples":
		return []string{"example_roots"} // This would be populated from actual example roots
	case "xmlui_search_howto":
		return []string{"docs/public/pages/howto.md"}
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
