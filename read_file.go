package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewReadFileTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_read_file",
		mcp.WithDescription("Reads a .mdx or .tsx or .scss file from the XMLUI source or docs tree."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Relative path under docs/pages/components or xmlui/src/components, e.g. 'xmlui/src/components/Spinner/Spinner.tsx'"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		relPath, ok := req.Params.Arguments["path"].(string)
		if !ok || relPath == "" {
			return mcp.NewToolResultError("Missing or invalid 'path' parameter"), nil
		}

		// Validate extension
		if !strings.HasSuffix(relPath, ".mdx") && !strings.HasSuffix(relPath, ".tsx") && !strings.HasSuffix(relPath, ".scss") {
			return mcp.NewToolResultError("Only .mdx and .tsx and .scss files are supported"), nil
		}

		// Construct and validate full path
		fullPath := filepath.Join(homeDir, relPath)
		if !strings.HasPrefix(fullPath, filepath.Join(homeDir, "docs", "pages", "components")) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, "xmlui", "src", "components")) {
			return mcp.NewToolResultError("Path not allowed. Must be under docs/pages/components or xmlui/src/components."), nil
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}
