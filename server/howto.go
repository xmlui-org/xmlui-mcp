package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		mcp.WithDescription("List all 'How To' entry titles from the howto directory."),
	)
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// List howto files from the howto directory
		paths := GetRepoPaths(xmluiDir)
		howtoDir := filepath.Join(xmluiDir, paths.Howto)
		WriteDebugLog("xmlui_list_howto: xmluiDir=%s, howtoDir=%s\n", xmluiDir, howtoDir)

		entries, err := os.ReadDir(howtoDir)
		if err != nil {
			WriteDebugLog("xmlui_list_howto: error reading howto dir: %v\n", err)
			return mcp.NewToolResultText("No how-to entries found."), nil
		}

		var rows []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
				continue
			}
			slug := strings.TrimSuffix(name, ".md")
			absPath := filepath.Join(howtoDir, name)
			relPath := filepath.Join("howto", name)
			title := documentTitle(absPath, relPath)
			url := HowtoURL(slug)
			// Each row is a followable citation: title, slug, URL (#17).
			rows = append(rows, fmt.Sprintf("- %s (%s) — %s", title, slug, url))
		}
		WriteDebugLog("xmlui_list_howto: found %d rows\n", len(rows))

		if len(rows) == 0 {
			return mcp.NewToolResultText("No how-to entries found."), nil
		}

		sort.Slice(rows, func(i, j int) bool {
			return strings.ToLower(rows[i]) < strings.ToLower(rows[j])
		})
		return mcp.NewToolResultText(strings.Join(rows, "\n")), nil
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
		paths := GetRepoPaths(xmluiDir)
		roots := []string{
			filepath.Join(xmluiDir, paths.Howto),
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

		human, err := ExecuteMediatedSearchWithAnalytics(ctx, "xmlui_search_howto", xmluiDir, cfg, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(human), nil
	}

	return tool, handler
}

// HowtoClassifier returns a classifier that identifies howto content.
func HowtoClassifier(homeDir string) func(rel string, absPath string) string {
	return func(rel string, absPath string) string {
		return "howtos" // everything in howto search is howto content
	}
}

// Helper functions for backwards compatibility
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
