package xmluimcp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetCacheDir(t *testing.T) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		t.Fatalf("GetCacheDir() failed: %v", err)
	}

	if cacheDir == "" {
		t.Fatal("GetCacheDir() returned empty string")
	}

	// Check that directory was created
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("Cache directory does not exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("Cache path is not a directory")
	}

	// Verify platform-specific paths
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			expected := filepath.Join(localAppData, "xmlui", "xmlui-mcp")
			if cacheDir != expected {
				t.Errorf("Expected cache dir %s, got %s", expected, cacheDir)
			}
		}
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			expected := filepath.Join(homeDir, "Library", "Caches", "xmlui", "xmlui-mcp")
			if cacheDir != expected {
				t.Errorf("Expected cache dir %s, got %s", expected, cacheDir)
			}
		}
	default:
		// Linux/Unix
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		homeDir, _ := os.UserHomeDir()
		var expected string
		if xdgCache != "" {
			expected = filepath.Join(xdgCache, "xmlui", "xmlui-mcp")
		} else if homeDir != "" {
			expected = filepath.Join(homeDir, ".cache", "xmlui", "xmlui-mcp")
		}
		if expected != "" && cacheDir != expected {
			t.Errorf("Expected cache dir %s, got %s", expected, cacheDir)
		}
	}
}

func TestGetReposDir(t *testing.T) {
	reposDir, err := GetReposDir()
	if err != nil {
		t.Fatalf("GetReposDir() failed: %v", err)
	}

	if reposDir == "" {
		t.Fatal("GetReposDir() returned empty string")
	}

	// Should end with /xmlui-repoes
	if filepath.Base(reposDir) != "xmlui-repoes" {
		t.Errorf("Expected repos directory to end with 'xmlui-repoes', got %s", reposDir)
	}

	// Parent should be the cache directory
	cacheDir, _ := GetCacheDir()
	expectedRepos := filepath.Join(cacheDir, "xmlui-repoes")
	if reposDir != expectedRepos {
		t.Errorf("Expected repos dir %s, got %s", expectedRepos, reposDir)
	}
}

func TestIsRepoValid(t *testing.T) {
	// Test with non-existent directory
	if isRepoValid("/nonexistent/path/to/repo") {
		t.Error("isRepoValid() should return false for non-existent directory")
	}

	// Create a temporary test directory
	tempDir := t.TempDir()

	// Test with empty directory
	if isRepoValid(tempDir) {
		t.Error("isRepoValid() should return false for empty directory")
	}

	// Create partial structure
	os.MkdirAll(filepath.Join(tempDir, "docs"), 0755)
	if isRepoValid(tempDir) {
		t.Error("isRepoValid() should return false when 'xmlui' directory is missing")
	}

	// Create essential directories
	os.MkdirAll(filepath.Join(tempDir, "xmlui"), 0755)

	// Now it should be valid
	if !isRepoValid(tempDir) {
		t.Error("isRepoValid() should return true when all requirements are met")
	}
}

func TestGetLatestXMLUITag(t *testing.T) {
	// This test requires internet connectivity
	// Skip if running in offline environment
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	version, zipURL, err := getLatestXMLUITag()

	// If it fails (e.g., rate limit, network issues), that's okay for a test
	if err != nil {
		t.Logf("getLatestXMLUITag() failed (expected in some cases): %v", err)
		t.Logf("This is expected if there are network issues or GitHub API rate limits")
		return
	}

	if version == "" {
		t.Error("Expected non-empty version")
	}

	if zipURL == "" {
		t.Error("Expected non-empty zip URL")
	}

	// Version should start with "xmlui@"
	if len(version) < 6 || version[:6] != "xmlui@" {
		t.Errorf("Expected version to start with 'xmlui@', got %s", version)
	}

	// ZIP URL should contain the version
	if !strings.Contains(zipURL, version) {
		t.Errorf("Expected ZIP URL to contain version %s, got %s", version, zipURL)
	}

	t.Logf("Latest version: %s", version)
	t.Logf("Download URL: %s", zipURL)
}
