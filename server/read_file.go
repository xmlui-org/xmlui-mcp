package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

		// Normalize first (#29): trim whitespace and clean "." / ".." segments
		// before anything downstream reasons about the path.
		relPath = filepath.ToSlash(filepath.Clean(strings.TrimSpace(relPath)))

		// Validate extension
		if !strings.HasSuffix(relPath, ".mdx") && !strings.HasSuffix(relPath, ".tsx") && !strings.HasSuffix(relPath, ".scss") && !strings.HasSuffix(relPath, ".md") {
			return mcp.NewToolResultError("Only .mdx, .tsx, .scss, and .md files are supported"), nil
		}

		// Then check the allowlist.
		fullPath := filepath.Join(homeDir, relPath)
		paths := GetRepoPaths(homeDir)
		allowedRoots := []string{paths.ComponentDocs, paths.ComponentSource, paths.ExtensionDocs, paths.Pages}
		var matchedRoot string
		for _, root := range allowedRoots {
			if strings.HasPrefix(fullPath, filepath.Join(homeDir, root)) {
				matchedRoot = root
				break
			}
		}
		if matchedRoot == "" {
			return mcp.NewToolResultError(fmt.Sprintf("Path not allowed. Must be under %s, %s, %s, or %s.", paths.ComponentDocs, paths.ComponentSource, paths.ExtensionDocs, paths.Pages)), nil
		}

		// Only then check existence, so the message reflects the actual problem.
		content, err := os.ReadFile(fullPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				msg := fmt.Sprintf("File not found: %s.", toRepoRelative(homeDir, fullPath))
				if listing := nearestExistingDirListing(fullPath, filepath.Join(homeDir, matchedRoot)); listing != "" {
					msg += " " + listing
				}
				return mcp.NewToolResultError(msg), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s: %v", toRepoRelative(homeDir, fullPath), errWithoutPath(err))), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	}

	return tool, handler
}

// nearestExistingDirListing walks up from the requested file's directory,
// bounded below by floorAbs (the allowed root that matched), until it finds
// a directory that actually exists, then formats its entries. This turns a
// wrong filename into a self-correcting suggestion in one round trip (#29).
func nearestExistingDirListing(fullPath, floorAbs string) string {
	dir := filepath.Dir(fullPath)
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return formatDirListing(dir)
		}
		if dir == floorAbs {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir || len(parent) < len(floorAbs) {
			return ""
		}
		dir = parent
	}
}

// formatDirListing renders up to 10 alphabetical entry names for dir,
// noting how many more exist when truncated. Never includes dir itself,
// so the absolute host path is never echoed back (#29).
func formatDirListing(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return ""
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	const maxEntries = 10
	truncated := len(names) > maxEntries
	if truncated {
		names = names[:maxEntries]
	}

	if truncated {
		return fmt.Sprintf("Nearby: %s (and %d more).", strings.Join(names, ", "), len(entries)-maxEntries)
	}
	return fmt.Sprintf("Nearby: %s.", strings.Join(names, ", "))
}
