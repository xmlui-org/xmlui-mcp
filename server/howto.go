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

// Helper: Read all top-level howto files and aggregate their contents
func readAllHowtoFiles(howtoDir string) ([]string, error) {
	files, err := os.ReadDir(howtoDir)
	if err != nil {
		return nil, err
	}
	var docs []string
	for _, file := range files {
		if !file.IsDir() {
			path := filepath.Join(howtoDir, file.Name())
			data, err := os.ReadFile(path)
			if err == nil {
				docs = append(docs, string(data))
			}
		}
	}
	return docs, nil
}

// Helper: Parse a single document - handles both legacy (## sections) and new (# title) formats
func parseHowtoSections(doc string) ([]string, []string) {
	var sections []string
	var titles []string
	
	scanner := bufio.NewScanner(strings.NewReader(doc))
	
	// Check if this is a new-format file (starts with # title)
	firstLine := ""
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	
	if strings.HasPrefix(firstLine, "# ") {
		// New format: single file with # title
		title := strings.TrimSpace(firstLine[2:])
		sections = append(sections, doc)
		titles = append(titles, title)
		return sections, titles
	}
	
	// Legacy format: parse ## sections
	var current strings.Builder
	var currentTitle string
	
	// Reset scanner to beginning since we already read the first line
	scanner = bufio.NewScanner(strings.NewReader(doc))
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
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
	if current.Len() > 0 {
		sections = append(sections, current.String())
		titles = append(titles, currentTitle)
	}
	
	return sections, titles
}

// Helper: Parse multiple howto docs into sections
func parseHowtoSectionsMulti(docs []string) ([]string, []string) {
	var allSections []string
	var allTitles []string
	
	for _, doc := range docs {
		sections, titles := parseHowtoSections(doc)
		allSections = append(allSections, sections...)
		allTitles = append(allTitles, titles...)
	}
	
	return allSections, allTitles
}

// NewListHowtoTool returns the MCP tool and handler for listing howto titles
func NewListHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	howtoDir := filepath.Join(xmluiDir, "docs", "public", "pages", "howto")
	tool := mcp.NewTool(
		"xmlui_list_howto",
		mcp.WithDescription("List all 'How To' entry titles from docs/public/pages/howto/ (and legacy howto.md if present)."),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var docs []string
		// Try legacy howto.md
		legacyPath := filepath.Join(xmluiDir, "docs", "public", "pages", "howto.md")
		if data, err := os.ReadFile(legacyPath); err == nil {
			docs = append(docs, string(data))
		}
		// Add all top-level howto files
		if moreDocs, err := readAllHowtoFiles(howtoDir); err == nil {
			docs = append(docs, moreDocs...)
		}
	_, titles := parseHowtoSectionsMulti(docs)
	return mcp.NewToolResultText(strings.Join(titles, "\n")), nil
	}
	return tool, handler
}

// NewSearchHowtoTool returns the MCP tool and handler for searching howto entries
func NewSearchHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	howtoDir := filepath.Join(xmluiDir, "docs", "public", "pages", "howto")
	tool := mcp.NewTool(
		"xmlui_search_howto",
		mcp.WithDescription("Search for 'How To' entries in docs/public/pages/howto/ (and legacy howto.md if present) by keyword or phrase. Returns full markdown sections."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keyword or phrase to search for.")),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var docs []string
		// Try legacy howto.md
		legacyPath := filepath.Join(xmluiDir, "docs", "public", "pages", "howto.md")
		if data, err := os.ReadFile(legacyPath); err == nil {
			docs = append(docs, string(data))
		}
		// Add all top-level howto files
		if moreDocs, err := readAllHowtoFiles(howtoDir); err == nil {
			docs = append(docs, moreDocs...)
		}
		sections, titles := parseHowtoSectionsMulti(docs)
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
