package server

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
			mcp.Description("Relative path under component docs, component source, extension docs, or pages directories"),
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
		paths := GetRepoPaths(homeDir)
		if !strings.HasPrefix(fullPath, filepath.Join(homeDir, paths.ComponentDocs)) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, paths.ComponentSource)) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, paths.ExtensionDocs)) &&
			!strings.HasPrefix(fullPath, filepath.Join(homeDir, paths.Pages)) {
			return mcp.NewToolResultError(fmt.Sprintf("Path not allowed. Must be under %s, %s, %s, or %s.", paths.ComponentDocs, paths.ComponentSource, paths.ExtensionDocs, paths.Pages)), nil
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}
