package server

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildURLRegistryFromRepo tests building the registry from the actual
// cached XMLUI repo (if available). This is an integration-style test.
func TestBuildURLRegistryFromRepo(t *testing.T) {
	// Try to find the cached repo
	homeDir := os.Getenv("XMLUI_TEST_HOME")
	if homeDir == "" {
		// Try common locations
		candidates := []string{
			filepath.Join(os.Getenv("HOME"), "xmlui"),
			filepath.Join(os.Getenv("HOME"), "Library", "Caches", "xmlui", "xmlui-mcp", "xmlui-repos", "xmlui@0.12.2"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(c, "docs", "src", "Main.xmlui")); err == nil {
				homeDir = c
				break
			}
		}
	}
	if homeDir == "" {
		t.Skip("No XMLUI repo found; set XMLUI_TEST_HOME to run this test")
	}

	ResetURLRegistry()
	registry := GetURLRegistry(homeDir)

	// Check that we found some paths
	paths := registry.ValidPaths()
	if len(paths) == 0 {
		t.Fatal("Registry has no valid paths")
	}
	t.Logf("Registry has %d valid paths", len(paths))

	// Check known-good paths
	knownGood := []string{
		"/",
		"/components/Button",
		"/components/AppState",
		"/components/ModalDialog",
		"/components/Tabs",
		"/components/Stack",
		"/components/Table",
		"/components/Form",
		"/guides/markup",
		"/guides/scripting",
		"/guides/forms",
		"/howto/use-built-in-form-validation",
	}
	for _, p := range knownGood {
		if !registry.IsValidPath(p) {
			t.Errorf("Expected valid path %q not found in registry", p)
		}
	}

	// Check known-bad paths (these should NOT be in the registry)
	knownBad := []string{
		"/conditional-rendering",
		"/data-binding",
		"/event-handling",
		"/code-behind",
		"/theming",
		"/components/Dialog",
		"/components/TabStrip",
	}
	for _, p := range knownBad {
		if registry.IsValidPath(p) {
			t.Errorf("Bogus path %q should NOT be in registry", p)
		}
	}
}

func TestValidateURL(t *testing.T) {
	// Build a minimal registry for unit testing
	registry := &URLRegistry{
		validPaths: map[string]bool{
			"/":                  true,
			"/components/Button": true,
			"/guides/markup":     true,
		},
		baseURL: "https://docs.xmlui.org",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"https://docs.xmlui.org/components/Button", "https://docs.xmlui.org/components/Button"},
		{"https://docs.xmlui.org/guides/markup", "https://docs.xmlui.org/guides/markup"},
		{"https://docs.xmlui.org/bogus-page", ""},
		{"https://docs.xmlui.org/components/Dialog", ""},
		{"https://docs.xmlui.org", "https://docs.xmlui.org"},
	}

	for _, tc := range tests {
		result := registry.ValidateURL(tc.input)
		if result != tc.expected {
			t.Errorf("ValidateURL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestConstructValidatedDocURL(t *testing.T) {
	registry := &URLRegistry{
		validPaths: map[string]bool{
			"/components/Button":              true,
			"/howto/use-built-in-form-validation": true,
			"/guides/markup":                  true,
			"/styles-and-themes/layout-props": true,
		},
		baseURL: "https://docs.xmlui.org",
	}

	tests := []struct {
		filePath string
		expected string
	}{
		{"docs/content/components/Button.md", "https://docs.xmlui.org/components/Button"},
		{"docs/content/components/Dialog.md", ""}, // Dialog doesn't exist
		{"docs/content/pages/howto/use-built-in-form-validation.md", "https://docs.xmlui.org/howto/use-built-in-form-validation"},
		{"docs/content/pages/markup.md", "https://docs.xmlui.org/guides/markup"},
		{"docs/content/pages/styles-and-themes/layout-props.md", "https://docs.xmlui.org/styles-and-themes/layout-props"},
		{"xmlui/src/components/Button/Button.tsx", ""}, // source files don't have doc URLs
	}

	for _, tc := range tests {
		result := constructValidatedDocURL(tc.filePath, registry)
		if result != tc.expected {
			t.Errorf("constructValidatedDocURL(%q) = %q, want %q", tc.filePath, result, tc.expected)
		}
	}
}
