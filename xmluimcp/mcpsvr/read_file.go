package mcpsvr

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
		mcp.WithDescription("Reads a .mdx, .tsx, .scss, or .md file from the XMLUI source or docs tree."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Relative path under docs/content/components, xmlui/src/components, or docs/public/pages, e.g. 'xmlui/src/components/Spinner/Spinner.tsx'"),
		),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		relPath, ok := req.Params.Arguments["path"].(string)
		if !ok || relPath == "" {
			return mcp.NewToolResultError("Missing or invalid 'path' parameter"), nil
		}

		// Validate extension
		if !strings.HasSuffix(relPath, ".mdx") && !strings.HasSuffix(relPath, ".tsx") && !strings.HasSuffix(relPath, ".scss") && !strings.HasSuffix(relPath, ".md") {
			return mcp.NewToolResultError("Only .mdx, .tsx, .scss, and .md files are supported"), nil
		}

		// Construct and validate full path
		fullPath := filepath.Join(homeDir, relPath)
		if !strings.HasPrefix(fullPath, filepath.Join(homeDir, "docs", "content", "components")) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, "xmlui", "src", "components")) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, "docs", "public", "pages")) {
			return mcp.NewToolResultError("Path not allowed. Must be under docs/content/components, xmlui/src/components, or docs/public/pages."), nil
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}
