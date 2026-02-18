package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		mcp.WithDescription("Returns the Markdown documentation for a given XMLUI component from docs/content/components."),
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

		mdxPath := filepath.Join(homeDir, "docs", "content", "components", componentName+".md")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s: %v", componentName, err)), nil
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
		baseURL := "https://docs.xmlui.org/components"
		componentURL := baseURL + "/" + componentName
		contentWithURL := contentStr + "\n\n**Source:** " + componentURL

		return mcp.NewToolResultText(contentWithURL), nil
	}

	return tool, handler
}

// getComponentSupplement finds additional documentation for thin component docs.
func getComponentSupplement(homeDir string, componentName string) string {
	var supplement strings.Builder
	maxSupplement := 2000

	// Extract the bare component name (handle paths like "Stack/VStack")
	baseName := filepath.Base(componentName)

	// Check parent component map
	if parent, ok := parentComponentMap[baseName]; ok {
		parentPath := filepath.Join(homeDir, "docs", "content", "components", parent+".md")
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
		srcDir := filepath.Join(homeDir, "xmlui", "src", "components", baseName)
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
