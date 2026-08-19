package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Each xmlui_list_howto row must be a followable citation: title, slug, and
// URL, not a bare title (#17).
func TestListHowtoRowsAreActionable(t *testing.T) {
	ResetRepoPaths()
	root := t.TempDir()

	// No mcp-paths.json present, so GetRepoPaths falls back to legacy
	// detection: paths.Howto = "docs/content/pages/howto".
	howtoDir := filepath.Join(root, "docs", "content", "pages", "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"fill-remaining-vertical-space.md":   "# Make a child fill the remaining vertical space\nBody.\n",
		"build-a-fullscreen-modal-dialog.md": "# Build a fullscreen modal dialog\nBody.\n",
		"no-heading.md":                      "Just prose.\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(howtoDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, handler := NewListHowtoTool(root)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text

	wantLine := "- Make a child fill the remaining vertical space (fill-remaining-vertical-space) — https://www.xmlui.org/docs/howto/fill-remaining-vertical-space"
	if !strings.Contains(text, wantLine) {
		t.Fatalf("missing expected row:\n%s\ngot:\n%s", wantLine, text)
	}
	if !strings.Contains(text, "(build-a-fullscreen-modal-dialog)") ||
		!strings.Contains(text, "https://www.xmlui.org/docs/howto/build-a-fullscreen-modal-dialog") {
		t.Fatalf("missing expected slug/URL for fullscreen modal dialog:\n%s", text)
	}
	if !strings.Contains(text, "(no-heading)") {
		t.Fatalf("heading-less file must still be listed:\n%s", text)
	}

	modalIdx := strings.Index(text, "Build a fullscreen modal dialog")
	fillIdx := strings.Index(text, "Make a child fill the remaining vertical space")
	if modalIdx == -1 || fillIdx == -1 {
		t.Fatalf("expected titles not found in:\n%s", text)
	}
	if modalIdx >= fillIdx {
		t.Fatalf("expected title-sorted order (B before M), got:\n%s", text)
	}

	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") || !strings.Contains(line, " (") || !strings.Contains(line, " — https://") {
			t.Fatalf("row is not an actionable citation: %q", line)
		}
	}
}
