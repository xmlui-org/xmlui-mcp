package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// The listing must route agents to a tool that exists and must not present
// index files as components (#22).
func TestListComponentsGuidance(t *testing.T) {
	ResetRepoPaths()
	root := t.TempDir()
	componentDir := filepath.Join(root, "docs", "content", "components")
	if err := os.MkdirAll(componentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Button.md", "Slider.md", "_overview.md"} {
		if err := os.WriteFile(filepath.Join(componentDir, name), []byte("# doc"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, handler := NewListComponentsTool(root)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text

	if !strings.Contains(text, "xmlui_component_docs") {
		t.Fatalf("listing must reference the real tool name:\n%s", text)
	}
	if strings.Contains(text, "xmlui_docs ") || strings.Contains(text, "call xmlui_docs") {
		t.Fatalf("listing references nonexistent xmlui_docs:\n%s", text)
	}
	if strings.Contains(text, "_overview") {
		t.Fatalf("index file listed as a component:\n%s", text)
	}
	if !strings.Contains(text, "Button") || !strings.Contains(text, "Slider") {
		t.Fatalf("real components missing:\n%s", text)
	}
}
