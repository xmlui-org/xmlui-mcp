package server

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// URLRegistry holds the set of valid documentation URL paths built from the
// actual docs site structure: Page declarations in Main.xmlui, component .md
// files, and extension subdirectories.
type URLRegistry struct {
	validPaths map[string]bool // path -> true, e.g. "/components/Button"
	baseURL    string
}

var (
	globalRegistry     *URLRegistry
	globalRegistryOnce sync.Once
)

// GetURLRegistry returns the singleton URL registry, building it on first call.
func GetURLRegistry(homeDir string) *URLRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = buildURLRegistry(homeDir)
	})
	return globalRegistry
}

// IsValidPath checks whether a URL path (e.g. "/components/Button") exists.
func (r *URLRegistry) IsValidPath(urlPath string) bool {
	return r.validPaths[urlPath]
}

// ValidateURL returns the full URL if the path is valid, or empty string if not.
func (r *URLRegistry) ValidateURL(fullURL string) string {
	// Extract path from full URL
	path := fullURL
	if strings.HasPrefix(fullURL, r.baseURL) {
		path = strings.TrimPrefix(fullURL, r.baseURL)
	}
	if path == "" {
		path = "/"
	}
	if r.validPaths[path] {
		return fullURL
	}
	return ""
}

// ValidPaths returns all valid URL paths (for testing/debugging).
func (r *URLRegistry) ValidPaths() []string {
	paths := make([]string, 0, len(r.validPaths))
	for p := range r.validPaths {
		paths = append(paths, p)
	}
	return paths
}

// buildURLRegistry constructs the registry by scanning the repo.
func buildURLRegistry(homeDir string) *URLRegistry {
	registry := &URLRegistry{
		validPaths: make(map[string]bool),
		baseURL:    "https://docs.xmlui.org",
	}

	// 1. Parse Main.xmlui for <Page url="..."> declarations
	extractPageURLs(homeDir, registry)

	// 2. Scan component .md files
	extractComponentURLs(homeDir, registry)

	// 3. Scan extension subdirectories
	extractExtensionURLs(homeDir, registry)

	WriteDebugLog("URL registry built with %d valid paths\n", len(registry.validPaths))
	return registry
}

var pageURLRegex = regexp.MustCompile(`<Page\s+url="([^"]+)"`)
var redirectRegex = regexp.MustCompile(`<Redirect`)

// extractPageURLs parses docs/src/Main.xmlui for Page declarations.
func extractPageURLs(homeDir string, registry *URLRegistry) {
	mainPath := filepath.Join(homeDir, "docs", "src", "Main.xmlui")
	content, err := os.ReadFile(mainPath)
	if err != nil {
		WriteDebugLog("URL registry: failed to read Main.xmlui: %v\n", err)
		return
	}

	text := string(content)
	matches := pageURLRegex.FindAllStringSubmatchIndex(text, -1)

	for _, loc := range matches {
		url := text[loc[2]:loc[3]]

		// Skip wildcards and 404
		if strings.Contains(url, "*") || url == "/404" {
			continue
		}

		// Skip pages that contain a <Redirect> child
		pageStart := loc[0]
		pageEnd := strings.Index(text[pageStart:], "</Page>")
		if pageEnd >= 0 {
			pageContent := text[pageStart : pageStart+pageEnd]
			if redirectRegex.MatchString(pageContent) {
				continue
			}
		}

		registry.validPaths[url] = true
	}
}

// extractComponentURLs scans docs/content/components/*.md for component pages.
func extractComponentURLs(homeDir string, registry *URLRegistry) {
	componentsDir := filepath.Join(homeDir, "docs", "content", "components")
	entries, err := os.ReadDir(componentsDir)
	if err != nil {
		WriteDebugLog("URL registry: failed to read components dir: %v\n", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, "_") {
			componentName := strings.TrimSuffix(name, ".md")
			registry.validPaths["/components/"+componentName] = true
		}
	}
}

// extractExtensionURLs scans extension subdirectories under docs/content/components/
// for extension component pages (directories starting with "xmlui-").
func extractExtensionURLs(homeDir string, registry *URLRegistry) {
	componentsDir := filepath.Join(homeDir, "docs", "content", "components")
	entries, err := os.ReadDir(componentsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "xmlui-") {
			continue
		}
		extDir := filepath.Join(componentsDir, entry.Name())
		extFiles, err := os.ReadDir(extDir)
		if err != nil {
			continue
		}
		for _, f := range extFiles {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || strings.HasPrefix(f.Name(), "_") {
				continue
			}
			componentName := strings.TrimSuffix(f.Name(), ".md")
			registry.validPaths["/extensions/"+entry.Name()+"/"+componentName] = true
		}
	}
}

// constructValidatedDocURL builds a documentation URL from a file path and
// returns it only if the URL exists in the registry. Returns empty string
// if the URL is not valid.
func constructValidatedDocURL(filePath string, registry *URLRegistry) string {
	urlPath := strings.ReplaceAll(filePath, "\\", "/")

	var candidate string
	switch {
	case strings.HasPrefix(urlPath, "docs/content/components/"):
		// docs/content/components/Table.md -> /components/Table
		componentName := strings.TrimSuffix(filepath.Base(urlPath), ".md")
		candidate = "/components/" + componentName

	case strings.HasPrefix(urlPath, "docs/content/pages/howto/") || strings.HasPrefix(urlPath, "docs/public/pages/howto/"):
		// docs/content/pages/howto/paginate-a-list.md -> /howto/paginate-a-list
		howtoName := strings.TrimSuffix(filepath.Base(urlPath), ".md")
		candidate = "/howto/" + howtoName

	case strings.HasPrefix(urlPath, "docs/content/pages/") || strings.HasPrefix(urlPath, "docs/public/pages/"):
		// For pages, we need to check both direct and /guides/ prefixed paths
		pageName := strings.TrimSuffix(filepath.Base(urlPath), ".md")

		// Handle subdirectories like styles-and-themes/
		rel := ""
		if strings.HasPrefix(urlPath, "docs/content/pages/") {
			rel = strings.TrimPrefix(urlPath, "docs/content/pages/")
		} else {
			rel = strings.TrimPrefix(urlPath, "docs/public/pages/")
		}
		rel = strings.TrimSuffix(rel, ".md")

		// Try full relative path first (handles styles-and-themes/layout-props etc.)
		if registry.IsValidPath("/" + rel) {
			return registry.baseURL + "/" + rel
		}
		// Try with /guides/ prefix (markup -> /guides/markup)
		if registry.IsValidPath("/guides/" + pageName) {
			return registry.baseURL + "/guides/" + pageName
		}
		// Try direct path
		candidate = "/" + pageName

	default:
		return ""
	}

	if registry.IsValidPath(candidate) {
		return registry.baseURL + candidate
	}
	return ""
}

// ResetURLRegistry resets the singleton for testing purposes.
func ResetURLRegistry() {
	globalRegistryOnce = sync.Once{}
	globalRegistry = nil
}

// scanSitemapPaths reads a sitemap.xml and returns all URL paths.
// This is used as an alternative source of truth if available.
func scanSitemapPaths(homeDir string) map[string]bool {
	paths := make(map[string]bool)
	sitemapPath := filepath.Join(homeDir, "docs", "public", "sitemap.xml")

	f, err := os.Open(sitemapPath)
	if err != nil {
		return paths
	}
	defer f.Close()

	baseURL := "https://docs.xmlui.org"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "<loc>") && strings.HasSuffix(line, "</loc>") {
			url := strings.TrimPrefix(line, "<loc>")
			url = strings.TrimSuffix(url, "</loc>")
			if strings.HasPrefix(url, baseURL) {
				path := strings.TrimPrefix(url, baseURL)
				if path == "" {
					path = "/"
				}
				paths[path] = true
			}
		}
	}
	return paths
}
