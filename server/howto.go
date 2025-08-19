package server

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper: Fuzzy word matching - returns true if text matches query with word-based fuzzy logic
func fuzzyMatch(text, query string) bool {
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// If single word query, use simple contains check
	if len(queryWords) == 1 {
		return strings.Contains(textLower, queryLower)
	}

	// For multiple words, require ALL words to be present (AND logic)
	for _, word := range queryWords {
		if !strings.Contains(textLower, word) {
			return false
		}
	}
	return true
}

// Helper: Convert title to URL anchor
func titleToAnchor(title string) string {
	// Convert title to URL anchor
	anchor := strings.ToLower(title)
	anchor = strings.ReplaceAll(anchor, " ", "-")
	// Remove special characters, keep only letters, numbers, hyphens
	var result strings.Builder
	for _, r := range anchor {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

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
		"xmlui_list_howto",
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
		"xmlui_search_howto",
		mcp.WithDescription("Search for 'How To' entries in docs/public/pages/howto.md by keyword or phrase. Returns full markdown sections."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keyword or phrase to search for.")),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sections, titles, err := parseHowtoSections(howtoPath)
		if err != nil {
			return mcp.NewToolResultError("Failed to parse howto.md: " + err.Error()), nil
		}
		query, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}
		var matches []string
		for i, section := range sections {
			if fuzzyMatch(section, query) {
				baseURL := "https://docs.xmlui.com/howto"
				anchor := titleToAnchor(titles[i])
				url := baseURL + "#" + anchor
				matchWithURL := section + "\n\n**Source:** " + url
				matches = append(matches, matchWithURL)
			}
		}
		if len(matches) == 0 {
			return mcp.NewToolResultText("No matches found."), nil
		}
		return mcp.NewToolResultText(strings.Join(matches, "\n---\n")), nil
	}
	return tool, handler
}
