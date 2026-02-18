package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	pagesRelPath string
	pagesOnce    sync.Once
)

// DetectPagesDir determines if the pages directory is at 'docs/content/pages' (new)
// or 'docs/public/pages' (old). It returns the relative path using forward slashes.
func DetectPagesDir(homeDir string) string {
	pagesOnce.Do(func() {
		// Check for new location: docs/content/pages
		newPath := filepath.Join(homeDir, "docs", "content", "pages")
		if _, err := os.Stat(newPath); err == nil {
			pagesRelPath = "docs/content/pages"
			return
		}

		// Check for old location: docs/public/pages
		oldPath := filepath.Join(homeDir, "docs", "public", "pages")
		if _, err := os.Stat(oldPath); err == nil {
			pagesRelPath = "docs/public/pages"
			return
		}

		// Default to new location if neither found (or error)
		pagesRelPath = "docs/content/pages"
	})
	return pagesRelPath
}

// commonParent returns the longest common directory prefix of the given paths.
// If only one path is given, returns that path. If none, returns ".".
func commonParent(paths []string) string {
	if len(paths) == 0 {
		return "."
	}
	if len(paths) == 1 {
		return paths[0]
	}

	// Clean and split all paths
	splitPaths := make([][]string, len(paths))
	for i, p := range paths {
		splitPaths[i] = strings.Split(filepath.Clean(p), string(filepath.Separator))
	}

	// Find shortest path length
	minLen := len(splitPaths[0])
	for _, sp := range splitPaths[1:] {
		if len(sp) < minLen {
			minLen = len(sp)
		}
	}

	// Find common prefix
	commonLen := 0
	for i := 0; i < minLen; i++ {
		seg := splitPaths[0][i]
		allMatch := true
		for _, sp := range splitPaths[1:] {
			if sp[i] != seg {
				allMatch = false
				break
			}
		}
		if !allMatch {
			break
		}
		commonLen = i + 1
	}

	if commonLen == 0 {
		return "."
	}

	result := strings.Join(splitPaths[0][:commonLen], string(filepath.Separator))
	// Preserve leading separator for absolute paths
	if filepath.IsAbs(paths[0]) && !filepath.IsAbs(result) {
		result = string(filepath.Separator) + result
	}
	return result
}
