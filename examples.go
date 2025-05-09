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

func NewExamplesTool(exampleRoots []string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_examples",
		mcp.WithDescription("Searches local sample apps for usage examples of XMLUI components. Provide a query string to search for. Optionally bias results by component name."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search term, e.g. 'Spinner', 'AppState', or 'delay=\"1000\"'")),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log := func(msg string, args ...any) {
			fmt.Fprintf(os.Stderr, msg+"\n", args...)
		}

		log("📥 Received arguments: %+v", req.Params.Arguments)

		rawQuery, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(rawQuery) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}
		query := strings.ToLower(rawQuery)

		component, _ := req.Params.Arguments["component"].(string)
		component = strings.ToLower(component)

		type exampleHit struct {
			Label   string   `json:"label"`
			Path    string   `json:"path"`
			Matches []string `json:"matches"`
			Content string   `json:"content"`
		}

		hits := []exampleHit{}
		seen := 0
		maxTotal := 50
		maxPerFile := 6

		extensions := map[string]bool{".tsx": true, ".xmlui": true, ".mdx": true}

		log("🔍 Starting search for: %q (component bias: %q)", query, component)
		for _, root := range exampleRoots {
			log("📁 Walking root: %s", root)

			err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					log("⚠️  Error accessing %s: %v", path, err)
					return nil
				}
				if d.IsDir() {
					return nil
				}

				ext := strings.ToLower(filepath.Ext(path))
				if !extensions[ext] {
					return nil
				}

				log("📄 Checking file: %s", path)

				file, err := os.Open(path)
				if err != nil {
					log("❌ Could not open %s: %v", path, err)
					return nil
				}
				defer file.Close()

				var lines []string
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}

				var found []string
				contextBefore := 10
				contextAfter := 10

				for i, line := range lines {
					lower := strings.ToLower(line)
					if strings.Contains(lower, query) || (component != "" && strings.Contains(lower, component)) {
						start := i - contextBefore
						if start < 0 {
							start = 0
						}
						end := i + contextAfter + 1
						if end > len(lines) {
							end = len(lines)
						}
						for j := start; j < end; j++ {
							found = append(found, fmt.Sprintf("%5d: %s", j+1, strings.TrimSpace(lines[j])))
						}
						found = append(found, "") // blank line between blocks

						if len(found) >= maxPerFile*contextAfter {
							break
						}
					}
				}

				if len(found) > 0 {
					relPath := path
					for _, rootBase := range exampleRoots {
						if strings.HasPrefix(path, rootBase) {
							if rel, err := filepath.Rel(rootBase, path); err == nil {
								relPath = filepath.Join(filepath.Base(rootBase), rel)
								break
							}
						}
					}

					fullContentBytes, err := os.ReadFile(path)
					if err != nil {
						log("❌ Failed to read full content of %s: %v", path, err)
						return nil
					}

					log("✅ Match in %s (%d lines)", relPath, len(found))

					hits = append(hits, exampleHit{
						Label:   fmt.Sprintf("Matches in %s", relPath),
						Path:    relPath,
						Matches: found,
						Content: string(fullContentBytes),
					})
					seen += len(found)
				}

				if seen >= maxTotal {
					log("🚦 Reached result limit (%d total lines)", seen)
					return filepath.SkipDir
				}
				return nil
			})

			if err != nil {
				log("❌ Walk error in root %s: %v", root, err)
			}
		}

		if len(hits) == 0 {
			log("❌ No matches found.")
			return mcp.NewToolResultText("No examples found."), nil
		}

		log("✅ Search complete. Total matched lines: %d", seen)
		return &mcp.CallToolResult{
			Result: hits,
		}, nil
	}

	return tool, handler
}
