package server

import (
	"os"
	"path/filepath"
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
