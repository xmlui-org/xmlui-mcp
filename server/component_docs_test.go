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

// componentDocsSectionMemberRequest builds an mcp.CallToolRequest carrying
// 'component' plus optional 'section' / 'member' arguments (empty strings are
// omitted, matching how a real MCP client would simply not send the key).
func componentDocsSectionMemberRequest(component, section, member string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	args := map[string]interface{}{"component": component}
	if section != "" {
		args["section"] = section
	}
	if member != "" {
		args["member"] = member
	}
	req.Params.Arguments = args
	return req
}

// setupComponentDocsSectionFixture creates a temp repo root with a single
// "List" component doc built from real-page anatomy (Properties / Events /
// Exposed Methods / Styling, each with anchor-tagged headings and ### member
// blocks) so section/member extraction (#26) can be exercised against
// something structurally close to a real reference page.
func setupComponentDocsSectionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs", "content", "components")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# List

The List displays rows.

## Properties [#properties]

### ` + "`data`" + ` [#data]

The data prop.

### ` + "`itemTemplate`" + ` [#itemtemplate]

Template prop.

## Events [#events]

### ` + "`didLoad`" + ` [#didload]

Fires on load.

## Exposed Methods [#exposed-methods]

### ` + "`scrollToTop`" + ` [#scrolltotop]

This method scrolls the list to the top.
**Signature**: ` + "`scrollToTop(): void`" + `

## Styling [#styling]

Theme vars here.
`
	if err := os.WriteFile(filepath.Join(docsDir, "List.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ResetRepoPaths()
	t.Cleanup(ResetRepoPaths)
	return root
}

func componentDocsResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	return result.Content[0].(mcp.TextContent).Text
}

func TestComponentDocsNoParamsReturnsFullPage(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", "", ""))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}
	text := componentDocsResultText(t, result)

	for _, heading := range []string{"## Properties", "## Events", "## Exposed Methods", "## Styling"} {
		if !strings.Contains(text, heading) {
			t.Fatalf("expected full page to contain %q, got: %s", heading, text)
		}
	}
}

func TestComponentDocsSectionProperties(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	for _, section := range []string{"properties", "props"} {
		result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", section, ""))
		if err != nil {
			t.Fatalf("section=%s: handler returned error: %v", section, err)
		}
		if result.IsError {
			t.Fatalf("section=%s: expected success, got error result: %+v", section, result)
		}
		text := componentDocsResultText(t, result)

		if !strings.Contains(text, "data") || !strings.Contains(text, "itemTemplate") {
			t.Fatalf("section=%s: expected properties section content, got: %s", section, text)
		}
		if strings.Contains(text, "didLoad") {
			t.Fatalf("section=%s: expected no Events content, got: %s", section, text)
		}
		if strings.Contains(text, "scrollToTop") {
			t.Fatalf("section=%s: expected no Exposed Methods content, got: %s", section, text)
		}
	}
}

func TestComponentDocsSectionMethodsAliases(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	for _, section := range []string{"methods", "apis"} {
		result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", section, ""))
		if err != nil {
			t.Fatalf("section=%s: handler returned error: %v", section, err)
		}
		if result.IsError {
			t.Fatalf("section=%s: expected success, got error result: %+v", section, result)
		}
		text := componentDocsResultText(t, result)

		if !strings.Contains(text, "scrollToTop(): void") {
			t.Fatalf("section=%s: expected scrollToTop signature, got: %s", section, text)
		}
		if strings.Contains(text, "The data prop") {
			t.Fatalf("section=%s: expected no Properties content, got: %s", section, text)
		}
	}
}

func TestComponentDocsSectionOverview(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", "overview", ""))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}
	text := componentDocsResultText(t, result)

	if !strings.Contains(text, "displays rows") {
		t.Fatalf("expected overview to contain the intro text, got: %s", text)
	}
	if strings.Contains(text, "## ") {
		t.Fatalf("expected overview to contain no '## ' sections, got: %s", text)
	}
}

func TestComponentDocsMemberWithoutSection(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", "", "scrollToTop"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}
	text := componentDocsResultText(t, result)

	if !strings.Contains(text, "scrollToTop(): void") {
		t.Fatalf("expected the scrollToTop signature, got: %s", text)
	}
	if strings.Contains(text, "The data prop") {
		t.Fatalf("expected no 'data' property content, got: %s", text)
	}
	if strings.Contains(text, "Fires on load") {
		t.Fatalf("expected no 'didLoad' event content, got: %s", text)
	}
	if len(text) >= 1024 {
		t.Fatalf("expected the member result to be well under 1KB for this fixture, got %d bytes: %s", len(text), text)
	}
}

func TestComponentDocsMemberMissListsAvailableMembersWithoutLeakingAbsolutePath(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", "", "nosuch"))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for a member miss, got: %+v", result)
	}
	text := componentDocsResultText(t, result)

	for _, name := range []string{"data", "itemTemplate", "didLoad", "scrollToTop"} {
		if !strings.Contains(text, name) {
			t.Fatalf("expected member miss message to list %q, got: %q", name, text)
		}
	}
	if strings.Contains(text, root) {
		t.Fatalf("member miss message leaked the absolute host path %q: %q", root, text)
	}
}

func TestComponentDocsSectionMissListsActualHeadings(t *testing.T) {
	root := setupComponentDocsSectionFixture(t)
	_, handler := NewComponentDocsTool(root)

	result, err := handler(context.Background(), componentDocsSectionMemberRequest("List", "nosuch", ""))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for a section miss, got: %+v", result)
	}
	text := componentDocsResultText(t, result)

	for _, heading := range []string{"Properties", "Events", "Exposed Methods", "Styling"} {
		if !strings.Contains(text, heading) {
			t.Fatalf("expected section miss message to list heading %q, got: %q", heading, text)
		}
	}
	if strings.Contains(text, root) {
		t.Fatalf("section miss message leaked the absolute host path %q: %q", root, text)
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
