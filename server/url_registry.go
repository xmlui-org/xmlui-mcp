package server

import (
	"path/filepath"
	"strings"
	"sync"
)

// baseURL is the root of the live XMLUI documentation. docs.xmlui.org is NOT
// it (that host serves the ModelContextProtocol site; deep paths are hard
// 404s) — see #16. Note the live site returns SPA shells with a 404 status on
// some real pages, so never "validate" these URLs by status code.
const baseURL = "https://www.xmlui.org/docs"

// URLRegistry maps file paths to documentation URLs.
// URLs are derived directly from file names — if a .md file exists, the page exists.
type URLRegistry struct {
	baseURL string
}

var (
	globalRegistry     *URLRegistry
	globalRegistryOnce sync.Once
)

// GetURLRegistry returns the singleton URL registry.
func GetURLRegistry(homeDir string) *URLRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = &URLRegistry{baseURL: baseURL}
	})
	return globalRegistry
}

// ValidateURL returns the URL as-is (no validation needed — if the file exists, the page exists).
func (r *URLRegistry) ValidateURL(fullURL string) string {
	return fullURL
}

// ResetURLRegistry resets the singleton for testing purposes.
func ResetURLRegistry() {
	globalRegistryOnce = sync.Once{}
	globalRegistry = nil
}

// ComponentURL returns the documentation URL for a component name.
func ComponentURL(name string) string {
	return baseURL + "/reference/components/" + name
}

// ExtensionURL returns the documentation URL for an extension component.
func ExtensionURL(pkg, name string) string {
	return baseURL + "/reference/extensions/" + pkg + "/" + name
}

// HowtoURL returns the documentation URL for a howto article.
func HowtoURL(slug string) string {
	return baseURL + "/howto/" + slug
}

// constructDocURL builds a documentation URL from a relative file path.
func constructDocURL(filePath string) string {
	p := strings.ReplaceAll(filePath, "\\", "/")

	switch {
	case containsSegment(p, "howto") && strings.HasSuffix(p, ".md"):
		slug := strings.TrimSuffix(filepath.Base(p), ".md")
		return HowtoURL(slug)

	case containsSegment(p, "components") && !containsSegment(p, "pages") && strings.HasSuffix(p, ".md"):
		name := strings.TrimSuffix(filepath.Base(p), ".md")
		return ComponentURL(name)

	case containsSegment(p, "extensions") && strings.HasSuffix(p, ".md"):
		// Extract package name from path like .../extensions/xmlui-animations/Animation.md
		parts := strings.Split(p, "/")
		for i, part := range parts {
			if part == "extensions" && i+2 < len(parts) {
				pkg := parts[i+1]
				name := strings.TrimSuffix(parts[i+2], ".md")
				return ExtensionURL(pkg, name)
			}
		}
		return ""

	case containsSegment(p, "pages") && strings.HasSuffix(p, ".md"):
		// Extract relative path from "pages/" onward
		idx := strings.Index(p, "pages/")
		if idx >= 0 {
			rel := p[idx+len("pages/"):]
			rel = strings.TrimSuffix(rel, ".md")
			return baseURL + "/" + rel
		}
		return ""

	default:
		return ""
	}
}

// containsSegment checks if a path contains a given directory segment.
func containsSegment(path, segment string) bool {
	return strings.Contains(path, "/"+segment+"/") || strings.HasPrefix(path, segment+"/")
}
