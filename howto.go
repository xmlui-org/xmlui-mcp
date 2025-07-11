package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper: Parse howto.md into sections (split on "## " heading)
func parseHowtoSections(howtoPath string) ([]string, []string, error) {
	file, err := os.Open(howtoPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var sections []string
	var titles []string
	var current strings.Builder
	var currentTitle string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if current.Len() > 0 {
				sections = append(sections, current.String())
				titles = append(titles, currentTitle)
				current.Reset()
			}
			currentTitle = strings.TrimSpace(line[3:])
		}
		if current.Len() > 0 || strings.HasPrefix(line, "## ") {
			current.WriteString(line + "\n")
		}
	}
	// Add last section
	if current.Len() > 0 {
		sections = append(sections, current.String())
		titles = append(titles, currentTitle)
	}
	return sections, titles, scanner.Err()
}

// NewListHowtoTool returns the MCP tool and handler for listing howto titles
func NewListHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	howtoPath := filepath.Join(xmluiDir, "docs", "public", "pages", "howto.md")
	tool := mcp.NewTool(
		"xmlui-list-howto",
		mcp.WithDescription("List all 'How To' entry titles from docs/public/pages/howto.md."),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		_, titles, err := parseHowtoSections(howtoPath)
		if err != nil {
			return mcp.NewToolResultError("Failed to parse howto.md: " + err.Error()), nil
		}
		return mcp.NewToolResultText(strings.Join(titles, "\n")), nil
	}
	return tool, handler
}

// NewSearchHowtoTool returns the MCP tool and handler for searching howto entries
func NewSearchHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	howtoPath := filepath.Join(xmluiDir, "docs", "public", "pages", "howto.md")
	tool := mcp.NewTool(
		"xmlui-search-howto",
		mcp.WithDescription("Search for 'How To' entries in docs/public/pages/howto.md by keyword or phrase. Returns full markdown sections."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keyword or phrase to search for.")),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sections, _, err := parseHowtoSections(howtoPath)
		if err != nil {
			return mcp.NewToolResultError("Failed to parse howto.md: " + err.Error()), nil
		}
		query, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}
		query = strings.ToLower(query)
		var matches []string
		for _, section := range sections {
			if strings.Contains(strings.ToLower(section), query) {
				matches = append(matches, section)
			}
		}
		if len(matches) == 0 {
			return mcp.NewToolResultText("No matches found."), nil
		}
		return mcp.NewToolResultText(strings.Join(matches, "\n---\n")), nil
	}
	return tool, handler
}
