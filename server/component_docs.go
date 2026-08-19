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

// parentComponentMap maps variant component names to their parent component.
// When a component's docs are thin, we look up the parent for supplemental content.
var parentComponentMap = map[string]string{
	"CVStack":        "Stack",
	"CHStack":        "Stack",
	"VStack":         "Stack",
	"HStack":         "Stack",
	"ModalDialog":    "Dialog",
	"AlertDialog":    "Dialog",
	"DropdownButton": "Button",
	"IconButton":     "Button",
	"ToggleButton":   "Button",
	"NumberBox":      "TextBox",
	"PasswordBox":    "TextBox",
	"SearchBox":      "TextBox",
}

func NewComponentDocsTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("xmlui_component_docs",
		mcp.WithDescription("Returns the Markdown documentation for a given XMLUI component."),
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

		// Normalize slash-qualified or extension-qualified args (e.g. "Stack/VStack",
		// "VStack.md") down to the bare component name the docs directory is keyed by (#29).
		componentName = normalizeComponentArg(componentName)
		if componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		paths := GetRepoPaths(homeDir)
		mdxPath := filepath.Join(homeDir, paths.ComponentDocs, componentName+".md")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return mcp.NewToolResultError(componentNotFoundMessage(homeDir, paths, componentName)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s under %s: %v", componentName, paths.ComponentDocs, errWithoutPath(err))), nil
		}

		contentStr := string(content)

		// Supplement thin docs (Rec #3)
		if len(contentStr) < 500 {
			supplement := getComponentSupplement(homeDir, componentName)
			if supplement != "" {
				contentStr += "\n\n---\n## Additional Context\n\n" + supplement
			}
		}

		// Add source URL
		componentURL := ComponentURL(componentName)
		contentWithURL := contentStr + "\n\n**Source:** " + componentURL

		return mcp.NewToolResultText(contentWithURL), nil
	}

	return tool, handler
}

// getComponentSupplement finds additional documentation for thin component docs.
func getComponentSupplement(homeDir string, componentName string) string {
	var supplement strings.Builder
	maxSupplement := 2000
	paths := GetRepoPaths(homeDir)

	// Extract the bare component name (handle paths like "Stack/VStack")
	baseName := filepath.Base(componentName)

	// Check parent component map
	if parent, ok := parentComponentMap[baseName]; ok {
		parentPath := filepath.Join(homeDir, paths.ComponentDocs, parent+".md")
		parentContent, err := os.ReadFile(parentPath)
		if err == nil {
			// Extract Properties and Events sections from parent
			extracted := extractSections(string(parentContent), []string{"Properties", "Events", "Props"})
			if extracted != "" {
				supplement.WriteString(fmt.Sprintf("*From parent component %s:*\n\n", parent))
				supplement.WriteString(extracted)
			}
		}
	}

	// Check for source .md or .tsx in xmlui/src/components/<Name>/
	if supplement.Len() < maxSupplement {
		srcDir := filepath.Join(homeDir, paths.ComponentSource, baseName)
		if entries, err := os.ReadDir(srcDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".tsx") {
					srcPath := filepath.Join(srcDir, name)
					data, err := os.ReadFile(srcPath)
					if err == nil {
						content := string(data)
						if len(content) > maxSupplement-supplement.Len() {
							content = content[:maxSupplement-supplement.Len()]
						}
						supplement.WriteString(fmt.Sprintf("\n*From source %s:*\n\n", name))
						supplement.WriteString(content)
					}
					break // only take first matching file
				}
			}
		}
	}

	result := supplement.String()
	if len(result) > maxSupplement {
		result = result[:maxSupplement]
	}
	return result
}

// extractSections extracts named markdown sections (## heading) from content.
func extractSections(content string, sectionNames []string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	capturing := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)
			matched := false
			for _, name := range sectionNames {
				if strings.EqualFold(heading, name) {
					matched = true
					break
				}
			}
			if matched {
				capturing = true
				result.WriteString(line + "\n")
				continue
			} else if capturing {
				// Hit a new section, stop capturing
				break
			}
		}
		if capturing {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

// normalizeComponentArg trims whitespace and collapses slash-qualified or
// extension-qualified component arguments (e.g. "Stack/VStack", "VStack.md")
// down to the bare name the flat component docs directory is keyed by (#29).
func normalizeComponentArg(raw string) string {
	name := strings.TrimSpace(raw)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".md")
	return strings.TrimSpace(name)
}

// componentNotFoundMessage builds an actionable miss message: it names the
// corpus-relative directory searched (never the absolute host path) and
// offers near-match suggestions plus the discovery tool (#29).
func componentNotFoundMessage(homeDir string, paths *RepoPaths, componentName string) string {
	msg := fmt.Sprintf("Component %q not found under %s.", componentName, paths.ComponentDocs)
	if suggestions := suggestComponentDocNames(homeDir, paths, componentName); len(suggestions) > 0 {
		msg += fmt.Sprintf(" Did you mean: %s?", strings.Join(suggestions, ", "))
	}
	msg += " Call xmlui_list_components to see all available component names."
	return msg
}

// suggestComponentDocNames returns up to 3 near matches for componentName
// drawn from the component docs directory the lookup itself reads (#29).
// Ranking is case-insensitive exact match first, then prefix match, then
// substring match; _-prefixed index files are skipped, matching the
// existing xmlui_list_components filtering convention.
func suggestComponentDocNames(homeDir string, paths *RepoPaths, componentName string) []string {
	dir := filepath.Join(homeDir, paths.ComponentDocs)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		if strings.HasPrefix(fileName, "_") || !strings.HasSuffix(fileName, ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(fileName, ".md"))
	}

	target := strings.ToLower(componentName)
	var exact, prefix, substr []string
	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case lower == target:
			exact = append(exact, name)
		case strings.HasPrefix(lower, target):
			prefix = append(prefix, name)
		case strings.Contains(lower, target):
			substr = append(substr, name)
		}
	}
	sort.Strings(exact)
	sort.Strings(prefix)
	sort.Strings(substr)

	suggestions := append(append(exact, prefix...), substr...)
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}
	return suggestions
}

// toRepoRelative strips the homeDir prefix from an absolute filesystem path
// so error messages never leak the host's absolute cache/checkout path
// (e.g. /Users/<name>/Library/Caches/xmlui/...) into model context (#29).
// Shared by component_docs.go and read_file.go.
func toRepoRelative(homeDir, path string) string {
	rel, err := filepath.Rel(homeDir, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// errWithoutPath extracts the underlying OS error from a *fs.PathError
// without its embedded absolute path, so wrapping "%v" around a read/stat
// error can't leak the host filesystem layout (#29). Shared by
// component_docs.go and read_file.go.
func errWithoutPath(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}
