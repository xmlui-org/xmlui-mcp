package mcpsvr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewListComponentsTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("xmlui_list_components",
		mcp.WithDescription("Lists all available XMLUI components based on .md files in docs/content/components."),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    BoolPtr(true),
		DestructiveHint: BoolPtr(false),
		IdempotentHint:  BoolPtr(true),
		OpenWorldHint:   BoolPtr(false),
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentRoot := filepath.Join(homeDir, "docs", "content", "components")
		components := []string{}

		err := filepath.WalkDir(componentRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip node_modules
			if d.IsDir() && d.Name() == "node_modules" {
				return filepath.SkipDir
			}

			// Only include .md files
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			rel, err := filepath.Rel(componentRoot, path)
			if err != nil {
				return err
			}

			parts := strings.Split(rel, string(filepath.Separator))
			base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))

			var component string
			if len(parts) == 2 && parts[0] == base {
				// Collapse "App/App" → "App"
				component = parts[0]
			} else {
				component = strings.TrimSuffix(rel, filepath.Ext(rel))
			}

			components = append(components, component)
			return nil
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to scan components: %v", err)), nil
		}

		sort.Strings(components)

		// Group by top-level subdirectory
		groups := make(map[string][]string)
		for _, c := range components {
			parts := strings.Split(c, string(filepath.Separator))
			group := parts[0]
			groups[group] = append(groups[group], c)
		}

		var out strings.Builder
		out.WriteString("Available XMLUI components:\n\n")
		groupNames := make([]string, 0, len(groups))
		for group := range groups {
			groupNames = append(groupNames, group)
		}
		sort.Strings(groupNames)

		for _, group := range groupNames {
			out.WriteString(fmt.Sprintf("## %s\n\n", group))
			for _, c := range groups[group] {
				name := filepath.Base(c)
				out.WriteString(fmt.Sprintf("- %s → call xmlui_docs with component: \"%s\"\n", name, c))
			}
			out.WriteString("\n")
		}

		return mcp.NewToolResultText(out.String()), nil
	}

	return tool, handler
}
