package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
)

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

// NewListHowtoTool returns the MCP tool and handler for listing howto titles
func NewListHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"xmlui_list_howto",
		mcp.WithDescription("List all 'How To' entry titles from docs/public/pages/howto/."),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Read all howto files from the howto directory
		howtoDir := filepath.Join(xmluiDir, "docs", "public", "pages", "howto")
		var docs []string
		if moreDocs, err := readAllHowtoFiles(howtoDir); err == nil {
			docs = append(docs, moreDocs...)
		}
		_, titles := parseHowtoSectionsMulti(docs)
		return mcp.NewToolResultText(strings.Join(titles, "\n")), nil
	}
	return tool, handler
}

// NewSearchHowtoTool wires xmlui_search_howto to the shared search mediator.
func NewSearchHowtoTool(xmluiDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"xmlui_search_howto",
		mcp.WithDescription("Search for 'How To' entries using a staged search mediator. Returns human-readable matches plus a JSON summary."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Keyword or phrase to search for.")),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}
		query := strings.TrimSpace(raw)

		// Howto search roots
		roots := []string{
			filepath.Join(xmluiDir, "docs", "public", "pages", "howto"),
		}

		cfg := MediatorConfig{
			Roots:                 roots,
			SectionKeys:           []string{"howtos"},
			PreferSections:        []string{"howtos"}, // bias towards howtos (though all are howtos)
			MaxResults:            50,
			FileExtensions:        []string{".md", ".mdx"},
			Stopwords:             DefaultStopwords(),
			Synonyms:              DefaultSynonyms(),
			Classifier:            HowtoClassifier(xmluiDir),
			EnableFilenameMatches: true,
		}

		human, _, err := ExecuteMediatedSearch(xmluiDir, cfg, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(human), nil
	}

	return tool, handler
}

// HowtoClassifier returns a classifier that identifies howto content.
func HowtoClassifier(homeDir string) func(rel string) string {
	return func(rel string) string {
		return "howtos" // everything in howto search is howto content
	}
}

// Helper functions for backwards compatibility
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

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

func parseHowtoSectionsMulti(docs []string) ([]string, []string) {
	var sections []string
	var titles []string
	for _, doc := range docs {
		var current strings.Builder
		var currentTitle string
		lines := strings.Split(doc, "\n")
		for _, line := range lines {
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
	}
	return sections, titles
}
