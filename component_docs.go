package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewComponentDocsTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("component_docs",
		mcp.WithDescription("Returns the Markdown documentation for a given XMLUI component from docs/pages/components."),
		mcp.WithString("component",
			mcp.Required(),
			mcp.Description("Component name, e.g. 'Button', 'Avatar', or 'Stack/VStack'"),
		),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentName, ok := req.Params.Arguments["component"].(string)
		if !ok || componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		mdxPath := filepath.Join(homeDir, "docs", "pages", "components", componentName+".mdx")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s: %v", componentName, err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}
