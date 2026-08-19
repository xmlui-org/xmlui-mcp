package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// componentDocsRequest builds an mcp.CallToolRequest carrying the given
// 'component' argument, matching the pattern in analytics_test.go's
// searchRequest.
func componentDocsRequest(component string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]interface{}{"component": component}
	return req
}

// setupComponentDocsFixture creates a temp repo root with a flat component
// docs directory (docs/content/components — the legacy default resolved by
// server/paths.go) containing a handful of component docs, and resets the
// RepoPaths singleton so the fixture is picked up fresh.
func setupComponentDocsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs", "content", "components")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Slider", "VStack", "Stack", "HStack"} {
		content := "# " + name + "\n\nDocs for " + name + ".\n"
		if err := os.WriteFile(filepath.Join(docsDir, name+".md"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ResetRepoPaths()
	t.Cleanup(ResetRepoPaths)
	return root
}

func TestComponentDocsNormalizesSlashQualifiedArg(t *testing.T) {
	root := setupComponentDocsFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsRequest("Stack/VStack"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "# VStack") {
		t.Fatalf("expected VStack doc content, got: %s", text)
	}
}

func TestComponentDocsMissSuggestsNearMatchesWithoutLeakingAbsolutePath(t *testing.T) {
	root := setupComponentDocsFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsRequest("Slid"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for a miss, got: %+v", result)
	}
	text := result.Content[0].(mcp.TextContent).Text

	if !strings.Contains(text, "Slider") {
		t.Fatalf("expected 'Slider' suggested in miss message, got: %q", text)
	}
	if !strings.Contains(text, "xmlui_list_components") {
		t.Fatalf("expected xmlui_list_components pointer in miss message, got: %q", text)
	}
	if strings.Contains(text, root) {
		t.Fatalf("miss message leaked the absolute host path %q: %q", root, text)
	}
}
