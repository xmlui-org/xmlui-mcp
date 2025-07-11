package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Helper: Fuzzy word matching - returns true if text matches query with word-based fuzzy logic
func fuzzyMatchExamples(text, query string) bool {
	textLower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)
	
	// If single word query, use simple contains check
	if len(queryWords) == 1 {
		return strings.Contains(textLower, queryLower)
	}
	
	// For multiple words, require ALL words to be present (AND logic)
	for _, word := range queryWords {
		if !strings.Contains(textLower, word) {
			return false
		}
	}
	return true
}

func NewExamplesTool(exampleRoots []string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	fmt.Fprintf(os.Stderr, "Example roots configured: %v\n", exampleRoots)

	tool := mcp.NewTool("xmlui_examples",
		mcp.WithDescription("Searches local sample apps for usage examples of XMLUI components. Provide a query string to search for."),
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

		type MatchingLine struct {
			LineNum int    `json:"lineNum"`
			Text    string `json:"text"`
		}

		type MatchResult struct {
			Path          string         `json:"path"`
			MatchingLines []MatchingLine `json:"matchingLines"`
			Content       string         `json:"content"`
			Score         int            `json:"-"`
		}

		results := []MatchResult{}
		matchCount := 0
		maxTotal := 50
		maxPerFile := 6

		extensions := map[string]bool{".tsx": true, ".xmlui": true, ".mdx": true}

		log("🔍 Starting search for: %q", query)
		for _, root := range exampleRoots {
			log("📁 Walking root: %s", root)

			err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					log("⚠️  Error accessing %s: %v", path, err)
					return nil
				}

				if d.IsDir() {
					if d.Name() == ".git" {
						return filepath.SkipDir
					}
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

				scanner := bufio.NewScanner(file)
				lineNum := 1
				var matchingLines []MatchingLine
				matchScore := 0

				for scanner.Scan() {
					line := scanner.Text()

					if fuzzyMatchExamples(line, query) {
						matchScore++
						matchingLines = append(matchingLines, MatchingLine{
							LineNum: lineNum,
							Text:    strings.TrimSpace(line),
						})
						matchCount++
						if len(matchingLines) >= maxPerFile {
							break
						}
					}
					lineNum++
				}

				if len(matchingLines) > 0 {
					relPath := path
					for _, rootBase := range exampleRoots {
						if strings.HasPrefix(path, rootBase) {
							if rel, err := filepath.Rel(rootBase, path); err == nil {
								relPath = filepath.Join(filepath.Base(rootBase), rel)
								break
							}
						}
					}

					fullContent, err := os.ReadFile(path)
					if err != nil {
						log("❌ Failed to read full content of %s: %v", path, err)
						return nil
					}

					log("✅ Match in %s (%d lines, score %d)", relPath, len(matchingLines), matchScore)
					results = append(results, MatchResult{
						Path:          relPath,
						MatchingLines: matchingLines,
						Content:       string(fullContent),
						Score:         matchScore,
					})
				}

				if matchCount >= maxTotal {
					log("🚦 Reached result limit (%d total matches)", matchCount)
					return filepath.SkipDir
				}
				return nil
			})

			if err != nil {
				log("❌ Walk error in root %s: %v", root, err)
			}
		}

		if len(results) == 0 {
			log("❌ No matches found.")
			return mcp.NewToolResultText("No examples found."), nil
		}

		log("✅ Search complete. Found %d files with %d matching lines", len(results), matchCount)

		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		var out strings.Builder
		out.WriteString(fmt.Sprintf("Found %d files matching query: %q\n\n", len(results), query))

		for _, result := range results {
			out.WriteString(fmt.Sprintf("## File: %s\n\n", result.Path))
			out.WriteString("### Matching Lines:\n\n")
			for _, line := range result.MatchingLines {
				out.WriteString(fmt.Sprintf("%5d: %s\n", line.LineNum, line.Text))
			}
			out.WriteString("\n### Complete File:\n\n```xml\n")
			out.WriteString(result.Content)
			out.WriteString("\n```\n\n")
		}

		return mcp.NewToolResultText(out.String()), nil
	}

	return tool, handler
}
