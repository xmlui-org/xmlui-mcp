package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RepoPaths holds resolved paths into the XMLUI repository.
// Loaded from mcp-paths.json if present, otherwise falls back to legacy defaults.
type RepoPaths struct {
	ComponentDocs   string `json:"componentDocs"`
	ComponentSource string `json:"componentSource"`
	ExtensionDocs   string `json:"extensionDocs"`
	ExtensionSource string `json:"extensionSource"`
	Pages           string `json:"pages"`
	Howto           string `json:"howto"`
	Blog            string `json:"blog"`
}

var (
	globalPaths     *RepoPaths
	globalPathsOnce sync.Once
)

// GetRepoPaths returns the resolved repo paths, loading them on first call.
func GetRepoPaths(homeDir string) *RepoPaths {
	globalPathsOnce.Do(func() {
		globalPaths = loadRepoPaths(homeDir)
	})
	return globalPaths
}

// ResetRepoPaths resets the singleton for testing.
func ResetRepoPaths() {
	globalPathsOnce = sync.Once{}
	globalPaths = nil
}

func loadRepoPaths(homeDir string) *RepoPaths {
	manifestPath := filepath.Join(homeDir, "mcp-paths.json")
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		var paths RepoPaths
		if parseErr := json.Unmarshal(data, &paths); parseErr == nil {
			WriteDebugLog("Loaded mcp-paths.json from %s\n", manifestPath)
			validatePaths(homeDir, &paths)
			return &paths
		} else {
			WriteDebugLog("Failed to parse mcp-paths.json: %v\n", parseErr)
		}
	}

	// Fallback to legacy paths for pre-manifest versions
	WriteDebugLog("No mcp-paths.json found, using legacy path detection\n")
	return detectLegacyPaths(homeDir)
}

func detectLegacyPaths(homeDir string) *RepoPaths {
	// Detect pages directory (old logic from DetectPagesDir)
	pagesPath := "docs/content/pages"
	if _, err := os.Stat(filepath.Join(homeDir, pagesPath)); err != nil {
		oldPath := "docs/public/pages"
		if _, err := os.Stat(filepath.Join(homeDir, oldPath)); err == nil {
			pagesPath = oldPath
		}
	}

	paths := &RepoPaths{
		ComponentDocs:   "docs/content/components",
		ComponentSource: "xmlui/src/components",
		ExtensionDocs:   "docs/content/components", // extensions were under components in legacy
		ExtensionSource: "packages",
		Pages:           pagesPath,
		Howto:           pagesPath + "/howto",
		Blog:            "blog",
	}
	validatePaths(homeDir, paths)
	return paths
}

func validatePaths(homeDir string, paths *RepoPaths) {
	check := func(name, rel string) {
		full := filepath.Join(homeDir, rel)
		if _, err := os.Stat(full); err != nil {
			WriteDebugLog("WARNING: mcp-paths %q -> %q not found. xmlui-mcp may need updating.\n", name, rel)
			fmt.Fprintf(os.Stderr, "WARNING: path %q (%s) not found — xmlui-mcp may need updating for this xmlui version\n", name, rel)
		}
	}
	check("componentDocs", paths.ComponentDocs)
	check("componentSource", paths.ComponentSource)
	check("extensionDocs", paths.ExtensionDocs)
	check("extensionSource", paths.ExtensionSource)
	check("pages", paths.Pages)
	check("howto", paths.Howto)
	check("blog", paths.Blog)
}
