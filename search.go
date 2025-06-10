package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewSearchTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("xmlui_search",
		mcp.WithDescription("Searches XMLUI source and documentation files (.mdx, .tsx, and .md) in docs/content/components, docs/public/pages, and xmlui/src/components. Use this to find references and implementations."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search term, e.g. 'Spinner' or 'useEffect'")),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}

		query = strings.ToLower(query)
		results := []string{}

		searchRoots := []string{
			filepath.Join(homeDir, "docs", "content", "components"),
			filepath.Join(homeDir, "docs", "public", "pages"),
			filepath.Join(homeDir, "xmlui", "src", "components"),
		}

		for _, root := range searchRoots {
			filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}

				if d.IsDir() {
					if d.Name() == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}

				if !(strings.HasSuffix(d.Name(), ".mdx") || strings.HasSuffix(d.Name(), ".tsx") || strings.HasSuffix(d.Name(), ".md")) {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				lineNum := 1
				for scanner.Scan() {
					line := scanner.Text()
					if strings.Contains(strings.ToLower(line), query) {
						rel, _ := filepath.Rel(homeDir, path)
						results = append(results, fmt.Sprintf("%s:%d: %s", rel, lineNum, line))
						if len(results) >= 50 {
							return nil
						}
					}
					lineNum++
				}
				return nil
			})
		}

		if len(results) == 0 {
			return mcp.NewToolResultText("No matches found."), nil
		}
		return mcp.NewToolResultText(strings.Join(results, "\n")), nil
	}

	return tool, handler
}
