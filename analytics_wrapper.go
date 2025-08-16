package main

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper function to extract session ID from context or request
func extractSessionID(ctx context.Context, req mcp.CallToolRequest) string {
	// Try to get session ID from context first
	if sessionID, ok := ctx.Value("session_id").(string); ok && sessionID != "" {
		return sessionID
	}
	
	// Try to get session ID from arguments
	if req.Params.Arguments != nil {
		if sessionID, ok := req.Params.Arguments["session_id"].(string); ok && sessionID != "" {
			return sessionID
		}
	}
	
	// Default to anonymous
	return "anonymous"
}

// Wrapper function to add analytics to any tool handler
func withAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		sessionID := extractSessionID(ctx, req)
		
		// Call the original handler
		result, err := handler(ctx, req)
		
		// Calculate metrics
		duration := time.Since(start)
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
		
		// Log the invocation
		LogTool(toolName, req.Params.Arguments, success, duration, resultSize, errorMsg, sessionID)
		
		return result, err
	}
}

// Special wrapper for search tools to capture additional search-specific metrics
func withSearchAnalytics(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		sessionID := extractSessionID(ctx, req)
		
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
		duration := time.Since(start)
		success := err == nil && result != nil
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
		
		// Log both general tool usage and specific search metrics
		LogTool(toolName, req.Params.Arguments, success, duration, resultSize, errorMsg, sessionID)
		
		// Log search-specific data
		searchPaths := getSearchPaths(toolName)
		LogSearch(toolName, query, resultCount, success, duration, sessionID, searchPaths)
		
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
