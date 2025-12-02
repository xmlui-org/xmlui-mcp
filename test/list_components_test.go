package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpsvr"
)

func TestNewListComponentsTool(t *testing.T) {
	// Skip if network tests are disabled (we need the repo)
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Ensure we have the XMLUI repo downloaded
	logger := getTestLogger(t)
	xmluiDir, err := xmluimcp.EnsureXMLUIRepo(logger)
	if err != nil {
		t.Fatalf("Failed to ensure XMLUI repo: %v", err)
	}

	// Create the tool
	tool, handler := mcpsvr.NewListComponentsTool(xmluiDir)

	// Verify tool was created
	if tool.Name != "xmlui_list_components" {
		t.Errorf("Expected tool name 'xmlui_list_components', got '%s'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool should have a description")
	}

	// Create a request
	req := mcp.CallToolRequest{}
	req.Params.Name = "xmlui_list_components"
	req.Params.Arguments = map[string]interface{}{}

	// Call the handler
	ctx := context.Background()
	result, err := handler(ctx, req)

	// Verify the call succeeded
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Handler returned nil result")
	}

	// Verify result has content
	if len(result.Content) == 0 {
		t.Error("Result should have content")
	}

	// Verify content is text
	var textContent string
	switch c := result.Content[0].(type) {
	case *mcp.TextContent:
		textContent = c.Text
	case mcp.TextContent:
		textContent = c.Text
	default:
		t.Fatalf("First content item should be TextContent, got %T", result.Content[0])
	}

	// Verify content contains expected text
	if len(textContent) == 0 {
		t.Error("Text content should not be empty")
	}

	// Should contain "Available XMLUI components"
	if !contains(textContent, "Available XMLUI components") {
		t.Error("Text should contain 'Available XMLUI components'")
	}

	t.Logf("Components list length: %d characters", len(textContent))
}

func TestNewListComponentsTool_WithAnalytics(t *testing.T) {
	// Skip if network tests are disabled (we need the repo)
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Ensure we have the XMLUI repo downloaded
	logger := getTestLogger(t)
	xmluiDir, err := xmluimcp.EnsureXMLUIRepo(logger)
	if err != nil {
		t.Fatalf("Failed to ensure XMLUI repo: %v", err)
	}

	// Create the tool
	_, handler := mcpsvr.NewListComponentsTool(xmluiDir)

	// Wrap with analytics (this is where the panic occurs)
	wrappedHandler := mcpsvr.WithAnalytics("xmlui_list_components", handler, logger)

	// Create a request
	req := mcp.CallToolRequest{}
	req.Params.Name = "xmlui_list_components"
	req.Params.Arguments = map[string]interface{}{}

	// Call the wrapped handler - this would have triggered the panic before the fix
	ctx := context.Background()
	result, err := wrappedHandler(ctx, req)

	// Verify the call succeeded
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Handler returned nil result")
	}

	// Verify result has content
	if len(result.Content) == 0 {
		t.Error("Result should have content")
	}
}

func TestNewListComponentsTool_WithMissingDirectory(t *testing.T) {
	// Test with a non-existent directory
	nonExistentDir := filepath.Join(os.TempDir(), "xmlui-test-nonexistent")

	// Create the tool
	_, handler := mcpsvr.NewListComponentsTool(nonExistentDir)

	// Create a request
	req := mcp.CallToolRequest{}
	req.Params.Name = "xmlui_list_components"
	req.Params.Arguments = map[string]interface{}{}

	// Call the handler
	ctx := context.Background()
	result, err := handler(ctx, req)

	// Should not return an error (errors are in the result)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Handler returned nil result")
	}

	// Should have error content
	if len(result.Content) == 0 {
		t.Error("Result should have content even on error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
