package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewXmluiDocsTool(docsDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_docs",
		mcp.WithDescription("Returns the documentation content for a given XMLUI component"),
		mcp.WithString("component",
			mcp.Required(),
			mcp.Description("Component name, e.g. 'Button' or 'Stack/VStack'"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentName, ok := req.Params.Arguments["component"].(string)
		if !ok || componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		// Construct path: docsDir/xmlui/src/components/{component}.mdx
		mdxPath := filepath.Join(docsDir, "xmlui", "src", "components", componentName+".mdx")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s: %v", componentName, err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}
