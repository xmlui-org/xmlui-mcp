package mcppkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewComponentDocsTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("xmlui_component_docs",
		mcp.WithDescription("Returns the Markdown documentation for a given XMLUI component from docs/content/components."),
		mcp.WithString("component",
			mcp.Required(),
			mcp.Description("Component name, e.g. 'Button', 'Avatar', or 'Stack/VStack'"),
		),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    BoolPtr(true),
		DestructiveHint: BoolPtr(false),
		IdempotentHint:  BoolPtr(true),
		OpenWorldHint:   BoolPtr(false),
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentName := RequestArgument(req, "component")
		if componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		mdxPath := filepath.Join(homeDir, "docs", "content", "components", componentName+".md")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s: %v", componentName, err)), nil
		}

		// Add source URL
		baseURL := "https://docs.xmlui.org/components"
		componentURL := baseURL + "/" + componentName
		contentWithURL := string(content) + "\n\n**Source:** " + componentURL

		return mcp.NewToolResultText(contentWithURL), nil
	}

	return tool, handler
}
