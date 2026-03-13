package server

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	ResetURLRegistry()
	registry := &URLRegistry{
		baseURL: "https://docs.xmlui.org",
	}

	// ValidateURL now always returns the URL as-is
	tests := []struct {
		input    string
		expected string
	}{
		{"https://docs.xmlui.org/components/Button", "https://docs.xmlui.org/components/Button"},
		{"https://docs.xmlui.org/guides/markup", "https://docs.xmlui.org/guides/markup"},
		{"https://docs.xmlui.org/bogus-page", "https://docs.xmlui.org/bogus-page"},
	}

	for _, tc := range tests {
		result := registry.ValidateURL(tc.input)
		if result != tc.expected {
			t.Errorf("ValidateURL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestConstructDocURL(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		// Component docs
		{"docs/content/components/Button.md", "https://docs.xmlui.org/components/Button"},
		{"website/content/docs/reference/components/Button.md", "https://docs.xmlui.org/components/Button"},

		// Howto
		{"docs/content/pages/howto/use-built-in-form-validation.md", "https://docs.xmlui.org/howto/use-built-in-form-validation"},
		{"website/content/docs/pages/howto/use-built-in-form-validation.md", "https://docs.xmlui.org/howto/use-built-in-form-validation"},

		// Pages
		{"docs/content/pages/styles-and-themes/layout-props.md", "https://docs.xmlui.org/styles-and-themes/layout-props"},
		{"website/content/docs/pages/styles-and-themes/layout-props.md", "https://docs.xmlui.org/styles-and-themes/layout-props"},

		// Extensions
		{"website/content/docs/reference/extensions/xmlui-animations/Animation.md", "https://docs.xmlui.org/extensions/xmlui-animations/Animation"},

		// Source files don't have doc URLs
		{"xmlui/src/components/Button/Button.tsx", ""},
	}

	for _, tc := range tests {
		result := constructDocURL(tc.filePath)
		if result != tc.expected {
			t.Errorf("constructDocURL(%q) = %q, want %q", tc.filePath, result, tc.expected)
		}
	}
}

func TestComponentURL(t *testing.T) {
	if got := ComponentURL("Button"); got != "https://docs.xmlui.org/components/Button" {
		t.Errorf("ComponentURL(Button) = %q", got)
	}
}

func TestExtensionURL(t *testing.T) {
	if got := ExtensionURL("xmlui-animations", "Animation"); got != "https://docs.xmlui.org/extensions/xmlui-animations/Animation" {
		t.Errorf("ExtensionURL = %q", got)
	}
}

func TestHowtoURL(t *testing.T) {
	if got := HowtoURL("use-built-in-form-validation"); got != "https://docs.xmlui.org/howto/use-built-in-form-validation" {
		t.Errorf("HowtoURL = %q", got)
	}
}
