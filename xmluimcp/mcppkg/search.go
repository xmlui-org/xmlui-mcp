package mcppkg

import (
	"context"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewSearchTool wires xmlui_search to the shared search mediator.
func NewSearchTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"xmlui_search",
		mcp.WithDescription("Searches XMLUI source, docs, and examples using a staged search mediator. Returns human-readable matches plus a JSON summary."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search term, e.g. 'Slider', 'boxShadow', or 'pathname context variable'")),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    BoolPtr(true),
		DestructiveHint: BoolPtr(false),
		IdempotentHint:  BoolPtr(true),
		OpenWorldHint:   BoolPtr(false),
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := RequestArgument(req, "query")
		if query == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}

		// Repository roots to scan (order matters for biasing)
		roots := []string{
			filepath.Join(homeDir, "docs", "content", "components"),
			filepath.Join(homeDir, "docs", "public", "pages"),
			filepath.Join(homeDir, "docs", "src", "components"),
			filepath.Join(homeDir, "xmlui", "src", "components"),
		}

		cfg := MediatorConfig{
			Roots:                 roots,
			SectionKeys:           []string{"components", "howtos", "examples", "source"},
			PreferSections:        []string{"components", "howtos"}, // bias docs/howtos when expanding
			MaxResults:            50,
			FileExtensions:        []string{".mdx", ".md", ".tsx", ".scss"},
			Stopwords:             DefaultStopwords(),
			Synonyms:              DefaultSynonyms(),
			Classifier:            SimpleClassifier(homeDir),
			EnableFilenameMatches: true,
		}

		human, _, err := ExecuteMediatedSearch(homeDir, cfg, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(human), nil
	}

	return tool, handler
}
