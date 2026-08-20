package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// readFileRequest builds an mcp.CallToolRequest carrying the given 'path'
// argument, matching the pattern in analytics_test.go's searchRequest.
func readFileRequest(path string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]interface{}{"path": path}
	return req
}

// setupReadFileFixture creates a temp repo root with a component source
// directory (xmlui/src/components/Slider — the legacy default resolved by
// server/paths.go) containing sibling files, and resets the RepoPaths
// singleton so the fixture is picked up fresh.
func setupReadFileFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	sliderDir := filepath.Join(root, "xmlui", "src", "components", "Slider")
	if err := os.MkdirAll(sliderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sliderDir, "Slider.tsx"), []byte("export const Slider = () => null;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sliderDir, "SliderReact.tsx"), []byte("export const SliderReact = () => null;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ResetRepoPaths()
	t.Cleanup(ResetRepoPaths)
	return root
}

func TestReadFileMissingFileListsSiblingsWithoutLeakingAbsolutePath(t *testing.T) {
	root := setupReadFileFixture(t)
	_, handler := NewReadFileTool(root)

	result, err := handler(context.Background(), readFileRequest("xmlui/src/components/Slider/SliderNative.tsx"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for a missing file, got: %+v", result)
	}
	text := result.Content[0].(mcp.TextContent).Text

	if !strings.Contains(text, "Slider.tsx") || !strings.Contains(text, "SliderReact.tsx") {
		t.Fatalf("expected sibling file names in not-found message, got: %q", text)
	}
	if strings.Contains(text, root) {
		t.Fatalf("not-found message leaked the absolute host path %q: %q", root, text)
	}
}

func TestReadFileDisallowedPrefixReturnsAllowlistMessage(t *testing.T) {
	root := setupReadFileFixture(t)
	_, handler := NewReadFileTool(root)

	result, err := handler(context.Background(), readFileRequest("components/Slider/SliderNative.tsx"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for a disallowed prefix, got: %+v", result)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Path not allowed") {
		t.Fatalf("expected allowlist message, got: %q", text)
	}
}

func TestReadFileAllowedExistingFileSucceeds(t *testing.T) {
	root := setupReadFileFixture(t)
	_, handler := NewReadFileTool(root)

	result, err := handler(context.Background(), readFileRequest("xmlui/src/components/Slider/Slider.tsx"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "export const Slider") {
		t.Fatalf("expected Slider.tsx content, got: %q", text)
	}
}

// Blog is a searched section, so it must be readable too (#4).
func TestReadFileAllowsBlogPath(t *testing.T) {
	ResetRepoPaths()
	root := t.TempDir()
	blogDir := filepath.Join(root, "blog", "public", "blog")
	if err := os.MkdirAll(blogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blogDir, "supabase-and-xmlui.md"), []byte("# Supabase and XMLUI\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, handler := NewReadFileTool(root)

	// Existing blog file reads successfully.
	result, err := handler(context.Background(), readFileRequest("blog/public/blog/supabase-and-xmlui.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Supabase and XMLUI") {
		t.Fatalf("blog file not readable: %s", text)
	}

	// Missing blog file gets the sibling listing, not the allowlist message.
	result, err = handler(context.Background(), readFileRequest("blog/public/blog/nope.md"))
	if err != nil {
		t.Fatal(err)
	}
	text = result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "Path not allowed") {
		t.Fatalf("blog path hit the allowlist message: %s", text)
	}
	if !strings.Contains(text, "supabase-and-xmlui.md") {
		t.Fatalf("sibling listing missing: %s", text)
	}

	// The allowlist message names blog among the allowed roots.
	result, err = handler(context.Background(), readFileRequest("secrets/nope.md"))
	if err != nil {
		t.Fatal(err)
	}
	text = result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Path not allowed") || !strings.Contains(text, "blog") {
		t.Fatalf("allowlist message must name blog: %s", text)
	}
}
