package xmluimcp

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpserver "xmlui-mcp/server"
)

const (
	githubAPIURL     = "https://api.github.com/repos/xmlui-org/xmlui/releases"
	fallbackVersion  = "xmlui@0.11.4"
	fallbackZipURL   = "https://github.com/xmlui-org/xmlui/archive/refs/tags/xmlui@0.11.4.zip"
	versionMarkerFile = ".xmlui-version"
)

// GitHubRelease represents a release from the GitHub API
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	CreatedAt  string `json:"created_at"`
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

	// Set User-Agent to avoid GitHub API rate limiting issues
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

// writeVersionMarker writes a version marker file to track which version is cached
func writeVersionMarker(repoDir, version string) error {
	markerPath := filepath.Join(repoDir, versionMarkerFile)
	return os.WriteFile(markerPath, []byte(version), 0644)
}

// readVersionMarker reads the version marker file
func readVersionMarker(repoDir string) (string, error) {
	markerPath := filepath.Join(repoDir, versionMarkerFile)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// isRepoValid checks if the cached repo exists and is valid
func isRepoValid(repoDir string) bool {
	// Check if directory exists
	info, err := os.Stat(repoDir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if version marker exists
	version, err := readVersionMarker(repoDir)
	if err != nil || version == "" {
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

// EnsureXMLUIRepo ensures the XMLUI repository is available in the cache
// Returns the path to the cached repository
func EnsureXMLUIRepo() (string, error) {
	repoDir, err := GetRepoDir()
	if err != nil {
		return "", fmt.Errorf("failed to get repo directory: %w", err)
	}

	// Check if repo is already valid
	if isRepoValid(repoDir) {
		mcpserver.WriteDebugLog("XMLUI repo already cached at: %s\n", repoDir)
		version, _ := readVersionMarker(repoDir)
		mcpserver.WriteDebugLog("Cached version: %s\n", version)
		return repoDir, nil
	}

	mcpserver.WriteDebugLog("XMLUI repo not found or invalid, downloading...\n")

	// Try to get the latest version from GitHub
	version, zipURL, err := getLatestXMLUITag()
	if err != nil {
		mcpserver.WriteDebugLog("Failed to get latest tag from GitHub: %v\n", err)
		mcpserver.WriteDebugLog("Falling back to version: %s\n", fallbackVersion)
		version = fallbackVersion
		zipURL = fallbackZipURL
	} else {
		mcpserver.WriteDebugLog("Latest version from GitHub: %s\n", version)
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

	// Ensure cleanup on failure
	success := false
	defer func() {
		if !success {
			os.RemoveAll(tempDir)
		}
	}()

	// Download the zip file
	zipPath := filepath.Join(tempDir, "xmlui.zip")
	mcpserver.WriteDebugLog("Downloading from: %s\n", zipURL)
	if err := downloadFile(zipURL, zipPath); err != nil {
		return "", fmt.Errorf("failed to download XMLUI repository: %w", err)
	}
	mcpserver.WriteDebugLog("Download complete\n")

	// Extract the zip file
	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	mcpserver.WriteDebugLog("Extracting archive...\n")
	if err := unzipFile(zipPath, extractDir); err != nil {
		return "", fmt.Errorf("failed to extract XMLUI repository: %w", err)
	}
	mcpserver.WriteDebugLog("Extraction complete\n")

	// GitHub zip files contain a single top-level directory
	// Find it and move its contents to the final location
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted directory: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("extracted archive is empty")
	}

	// Find the top-level directory (should be xmlui-org-xmlui-* or similar)
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

	// Create the final repo directory
	finalDir := filepath.Join(tempDir, "final")
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create final directory: %w", err)
	}

	// Move contents from top-level dir to final dir
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

	// Write version marker
	if err := writeVersionMarker(finalDir, version); err != nil {
		return "", fmt.Errorf("failed to write version marker: %w", err)
	}

	// Remove old repo directory if it exists
	if err := os.RemoveAll(repoDir); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove old repository: %w", err)
	}

	// Atomically move the final directory to the repo directory
	if err := os.Rename(finalDir, repoDir); err != nil {
		return "", fmt.Errorf("failed to move repository to final location: %w", err)
	}

	// Mark success so cleanup doesn't remove our work
	success = true

	// Clean up temp directory
	os.RemoveAll(tempDir)

	mcpserver.WriteDebugLog("XMLUI repo successfully cached at: %s\n", repoDir)
	mcpserver.WriteDebugLog("Version: %s\n", version)

	return repoDir, nil
}
