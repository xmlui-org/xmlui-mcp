package mcpsvr

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewExamplesTool wires xmlui_examples to the shared search mediator.
func NewExamplesTool(exampleRoots []string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	WriteDebugLog("Example roots configured: %v\n", exampleRoots)

	tool := mcp.NewTool("xmlui_examples",
		mcp.WithDescription("Searches local sample apps for usage examples of XMLUI components using a staged search mediator. Returns human-readable matches plus a JSON summary."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search term, e.g. 'Spinner', 'AppState', or 'delay=\"1000\"'")),
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

		// If no example roots configured, fall back to simple message
		if len(exampleRoots) == 0 {
			return mcp.NewToolResultText("No example directories configured."), nil
		}

		cfg := MediatorConfig{
			Roots:                 exampleRoots,
			SectionKeys:           []string{"examples"},
			PreferSections:        []string{"examples"}, // bias towards examples (though all are examples)
			MaxResults:            50,
			FileExtensions:        []string{".tsx", ".xmlui", ".mdx", ".md"},
			Stopwords:             DefaultStopwords(),
			Synonyms:              DefaultSynonyms(),
			Classifier:            ExamplesClassifier(),
			EnableFilenameMatches: true,
		}

		// Use a synthetic homeDir for relative paths in examples
		homeDir := "examples"
		human, _, err := ExecuteMediatedSearch(homeDir, cfg, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(human), nil
	}

	return tool, handler
}

// ExamplesClassifier returns a classifier that puts everything in "examples" section.
func ExamplesClassifier() func(rel string) string {
	return func(rel string) string {
		return "examples"
	}
}
