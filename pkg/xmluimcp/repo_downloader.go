package xmluimcp

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mcpserver "xmlui-mcp/server"
)

const (
	githubAPIURL   = "https://api.github.com/repos/xmlui-org/xmlui/releases"
	maxCachedRepos = 5
	metadataFile   = "metadata.json"
)

// downloadMutex prevents concurrent downloads within the same process
var downloadMutex sync.Mutex

// ErrVersionNotFound is returned when the requested version does not exist
var ErrVersionNotFound = errors.New("version not found")

// GitHubRelease represents a release from the GitHub API
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	CreatedAt  string `json:"created_at"`
}

// CacheMetadata stores access times for LRU eviction
type CacheMetadata struct {
	LastAccessTimeByRepo map[string]string `json:"lastAccessTimeByRepo"`
}

// getLatestXMLUITag queries GitHub API for the latest xmlui@* release
func getLatestXMLUITag() (string, string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	//From Github docs: "Requests with no User-Agent header will be rejected."
	req.Header.Set("User-Agent", "xmlui-mcp")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch releases from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	// Find the latest xmlui@* release
	for _, release := range releases {
		if strings.HasPrefix(release.TagName, "xmlui@") {
			// Construct the download URL
			zipURL := fmt.Sprintf("https://github.com/xmlui-org/xmlui/archive/refs/tags/%s.zip", release.TagName)
			return release.TagName, zipURL, nil
		}
	}

	return "", "", fmt.Errorf("no xmlui@* releases found")
}

// downloadFile downloads a file from the given URL to the destination path
func downloadFile(url, destPath string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrVersionNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create the destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy the response body to the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// unzipFile extracts a zip file to the destination directory
func unzipFile(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	// Extract all files
	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			// Create directory
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// isRepoValid checks if the cached repo exists and is valid
func isRepoValid(repoDir string) bool {
	// Check if directory exists
	info, err := os.Stat(repoDir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if essential directories exist
	docsDir := filepath.Join(repoDir, "docs")
	if _, err := os.Stat(docsDir); err != nil {
		return false
	}

	xmluiDir := filepath.Join(repoDir, "xmlui")
	if _, err := os.Stat(xmluiDir); err != nil {
		return false
	}

	return true
}

func updateMetadata(reposDir string, repoName string) error {
	metadataPath := filepath.Join(reposDir, metadataFile)

	// Read existing metadata
	var metadata CacheMetadata
	data, err := os.ReadFile(metadataPath)

	if err == nil {
		json.Unmarshal(data, &metadata)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if metadata.LastAccessTimeByRepo == nil {
		metadata.LastAccessTimeByRepo = make(map[string]string)
	}

	// Update access time
	metadata.LastAccessTimeByRepo[repoName] = time.Now().UTC().Format(time.RFC3339)

	// Write back
	newData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return os.WriteFile(metadataPath, newData, 0644)
}

func cleanupCache(reposDir string) error {
	metadataPath := filepath.Join(reposDir, metadataFile)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		// If metadata cannot be read, do not clear old repos
		return nil
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil // Invalid metadata, don't clear
	}

	// Collect existing repos
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return err
	}

	var repos []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "xmlui@") {
			repos = append(repos, entry.Name())
		}
	}

	if len(repos) <= maxCachedRepos {
		return nil
	}

	// Sort repos by access time
	type repoEntry struct {
		Name string
		Time time.Time
	}

	var sortedRepos []repoEntry
	for _, name := range repos {
		tsStr, ok := metadata.LastAccessTimeByRepo[name]
		var ts time.Time
		if ok {
			ts, _ = time.Parse(time.RFC3339, tsStr)
		}
		// If parsing fails or not found, it gets zero time (oldest)
		sortedRepos = append(sortedRepos, repoEntry{Name: name, Time: ts})
	}

	sort.Slice(sortedRepos, func(i, j int) bool {
		return sortedRepos[i].Time.Before(sortedRepos[j].Time)
	})

	// Delete oldest until we have maxCachedRepos
	toDelete := len(sortedRepos) - maxCachedRepos
	for i := range toDelete {
		repoName := sortedRepos[i].Name
		repoPath := filepath.Join(reposDir, repoName)
		mcpserver.WriteDebugLog("Removing old cache: %s\n", repoName)
		os.RemoveAll(repoPath)
		delete(metadata.LastAccessTimeByRepo, repoName)
	}

	// Save updated metadata
	newData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metadataPath, newData, 0644)
}

// EnsureXMLUIRepo ensures the XMLUI repository is available in the cache
// Returns the path to the cached repository
func EnsureXMLUIRepo(version string) (string, error) {
	reposDir, err := GetReposDir()
	if err != nil {
		return "", fmt.Errorf("failed to get repos directory: %w", err)
	}

	var tagName string
	var zipURL string

	if version == "" {
		// Fetch latest
		t, url, err := getLatestXMLUITag()
		if err == nil {
			tagName = t
			zipURL = url
		} else {
			// Try to fallback to cached version
			latestCached, cacheErr := getLatestCachedTag(reposDir)
			if cacheErr == nil && latestCached != "" {
				fmt.Fprintf(os.Stderr, "Warning: Failed to check the latest version of xmlui: %v\nFalling back to cached version: %s\n", err, latestCached)
				mcpserver.WriteDebugLog("Failed to fetch latest version from GitHub (%v), falling back to cached version: %s\n", err, latestCached)
				tagName = latestCached
			} else {
				return "", fmt.Errorf("failed to get latest version: %w", err)
			}
		}
	} else {
		// Use specified version
		if strings.HasPrefix(version, "xmlui@") {
			tagName = version
		} else {
			tagName = "xmlui@" + version
		}
		zipURL = fmt.Sprintf("https://github.com/xmlui-org/xmlui/archive/refs/tags/%s.zip", tagName)
	}

	repoDir := filepath.Join(reposDir, tagName)

	// Check if repo is already valid
	if isRepoValid(repoDir) {
		updateMetadata(reposDir, tagName)
		cleanupCache(reposDir)

		mcpserver.WriteDebugLog("XMLUI repo already cached at: %s\n", repoDir)
		return repoDir, nil
	}

	// Ensure parent dir exists
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create repos directory: %w", err)
	}

	// Acquire file-based lock for this specific repo version
	lockFile := repoDir + ".lock"
	mcpserver.WriteDebugLog("Acquiring file-based download lock: %s\n", lockFile)

	// Open or create lock file
	lock, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create lock file: %w", err)
	}
	defer lock.Close()

	// Acquire exclusive lock
	unlock, err := acquireFileLock(lock)
	if err != nil {
		os.Remove(lockFile)
		return "", fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer func() {
		unlock()
		os.Remove(lockFile)
	}()

	mcpserver.WriteDebugLog("File-based download lock acquired\n")

	// Check again after acquiring lock
	if isRepoValid(repoDir) {
		mcpserver.WriteDebugLog("XMLUI repo was cached by another process\n")
		updateMetadata(reposDir, tagName)
		cleanupCache(reposDir)
		return repoDir, nil
	}

	mcpserver.WriteDebugLog("Downloading XMLUI repo version %s...\n", tagName)

	if zipURL == "" {
		return "", fmt.Errorf("Cannot download repository. Expected to read it from cache, but it was invalidated while processing")
	}

	// Use a temporary directory for atomic download
	tempDir := repoDir + ".tmp"

	// Clean up any previous failed download attempts
	if err := os.RemoveAll(tempDir); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to clean up temporary directory: %w", err)
	}

	// Create temporary directory
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	success := false
	defer func() {
		if !success {
			os.RemoveAll(tempDir)
		}
	}()

	// Download zip
	zipPath := filepath.Join(tempDir, "xmlui.zip")
	mcpserver.WriteDebugLog("Downloading from: %s\n", zipURL)
	if err := downloadFile(zipURL, zipPath); err != nil {
		return "", fmt.Errorf("failed to download XMLUI repository: %w", err)
	}

	// Extract
	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	mcpserver.WriteDebugLog("Extracting archive...\n")
	if err := unzipFile(zipPath, extractDir); err != nil {
		return "", fmt.Errorf("failed to extract XMLUI repository: %w", err)
	}
	mcpserver.WriteDebugLog("Extraction complete\n")

	// Find top level dir
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted directory: %w", err)
	}

	var topLevelDir string
	for _, entry := range entries {
		if entry.IsDir() {
			topLevelDir = filepath.Join(extractDir, entry.Name())
			break
		}
	}

	if topLevelDir == "" {
		return "", fmt.Errorf("no top-level directory found in extracted archive")
	}

	// Move contents to final location structure inside tempDir
	finalDir := filepath.Join(tempDir, "final")
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create final directory: %w", err)
	}

	topEntries, err := os.ReadDir(topLevelDir)
	if err != nil {
		return "", fmt.Errorf("failed to read top-level directory: %w", err)
	}

	for _, entry := range topEntries {
		srcPath := filepath.Join(topLevelDir, entry.Name())
		destPath := filepath.Join(finalDir, entry.Name())
		if err := os.Rename(srcPath, destPath); err != nil {
			return "", fmt.Errorf("failed to move %s: %w", entry.Name(), err)
		}
	}

	// Atomically move to repoDir
	if err := os.RemoveAll(repoDir); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove old directory: %w", err)
	}

	if err := os.Rename(finalDir, repoDir); err != nil {
		return "", fmt.Errorf("failed to move repository to final location: %w", err)
	}

	success = true
	os.RemoveAll(tempDir)

	mcpserver.WriteDebugLog("XMLUI repo successfully cached at: %s\n", repoDir)

	// Update metadata and cleanup
	updateMetadata(reposDir, tagName)
	cleanupCache(reposDir)

	return repoDir, nil
}

// getLatestCachedTag finds the latest version in the cache
func getLatestCachedTag(reposDir string) (string, error) {
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return "", err
	}

	var tags []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "xmlui@") {
			fullPath := filepath.Join(reposDir, entry.Name())
			if isRepoValid(fullPath) {
				tags = append(tags, entry.Name())
			}
		}
	}

	if len(tags) == 0 {
		return "", nil
	}

	sort.Slice(tags, func(i, j int) bool {
		v1 := strings.TrimPrefix(tags[i], "xmlui@")
		v2 := strings.TrimPrefix(tags[j], "xmlui@")
		return compareVersions(v1, v2) < 0
	})

	return tags[len(tags)-1], nil
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := max(len(parts2), len(parts1))

	for i := range maxLen {
		var s1, s2 string
		if i < len(parts1) {
			s1 = parts1[i]
		}
		if i < len(parts2) {
			s2 = parts2[i]
		}

		n1, err1 := strconv.Atoi(s1)
		n2, err2 := strconv.Atoi(s2)

		if err1 == nil && err2 == nil {
			if n1 < n2 {
				return -1
			}
			if n1 > n2 {
				return 1
			}
		} else {
			if s1 < s2 {
				return -1
			}
			if s1 > s2 {
				return 1
			}
		}
	}

	return 0
}
