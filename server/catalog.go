package server

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CatalogFile is an in-memory snapshot of a file in the XMLUI repo or example tree.
// Search tools use this to avoid repeated filesystem walks and disk reads.
type CatalogFile struct {
	RelPath string
	AbsPath string
	Name    string
	Content string
	Lines   []string
}

// RepoCatalog caches the files under a set of roots in walk order.
type RepoCatalog struct {
	HomeDir   string
	RootPaths []string

	filesByRoot map[string][]CatalogFile
	filesByRel  map[string]CatalogFile
	filesByAbs  map[string]CatalogFile
}

// BuildRepoCatalog walks the provided roots once and caches their file contents.
// It is intentionally broad: the goal is to trade startup work for fast tool calls.
func BuildRepoCatalog(homeDir string, roots []string, allowedExtensions []string) (*RepoCatalog, error) {
	catalog := &RepoCatalog{
		HomeDir:     homeDir,
		RootPaths:   make([]string, 0, len(roots)),
		filesByRoot: make(map[string][]CatalogFile),
		filesByRel:  make(map[string]CatalogFile),
		filesByAbs:  make(map[string]CatalogFile),
	}

	allowed := make(map[string]struct{}, len(allowedExtensions))
	for _, ext := range allowedExtensions {
		allowed[strings.ToLower(ext)] = struct{}{}
	}

	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if cleanRoot == "." || cleanRoot == "" {
			continue
		}

		info, err := os.Stat(cleanRoot)
		if err != nil || !info.IsDir() {
			continue
		}

		catalog.RootPaths = append(catalog.RootPaths, cleanRoot)
		entries := make([]CatalogFile, 0)

		walkErr := filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}

			if len(allowed) > 0 {
				if _, ok := allowed[strings.ToLower(filepath.Ext(d.Name()))]; !ok {
					return nil
				}
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			rel, relErr := filepath.Rel(homeDir, path)
			if relErr != nil {
				rel = path
			}

			cf := CatalogFile{
				RelPath: rel,
				AbsPath: path,
				Name:    d.Name(),
				Content: string(data),
				Lines:   splitCatalogLines(string(data)),
			}

			entries = append(entries, cf)
			catalog.filesByAbs[cf.AbsPath] = cf
			catalog.filesByRel[filepath.Clean(cf.RelPath)] = cf
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("failed to index %s: %w", cleanRoot, walkErr)
		}

		catalog.filesByRoot[cleanRoot] = entries
	}

	return catalog, nil
}

// FilesForRoot returns the cached files for a root path.
func (c *RepoCatalog) FilesForRoot(root string) []CatalogFile {
	if c == nil {
		return nil
	}
	return c.filesByRoot[filepath.Clean(root)]
}

// FindByRel returns a cached file by relative path.
func (c *RepoCatalog) FindByRel(relPath string) (CatalogFile, bool) {
	if c == nil {
		return CatalogFile{}, false
	}
	file, ok := c.filesByRel[filepath.Clean(relPath)]
	return file, ok
}

// FindByAbs returns a cached file by absolute path.
func (c *RepoCatalog) FindByAbs(absPath string) (CatalogFile, bool) {
	if c == nil {
		return CatalogFile{}, false
	}
	file, ok := c.filesByAbs[filepath.Clean(absPath)]
	return file, ok
}

func splitCatalogLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	scanner := bufio.NewScanner(strings.NewReader(content))
	lines := make([]string, 0, 32)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
