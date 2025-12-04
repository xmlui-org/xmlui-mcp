package xmluimcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureXMLUIRepoIntegration tests the full integration of automatic repo download
func TestEnsureXMLUIRepoIntegration(t *testing.T) {
	// This test requires internet connectivity
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Get the repo directory (latest version)
	repoDir, err := EnsureXMLUIRepo("")
	if err != nil {
		t.Fatalf("EnsureXMLUIRepo(\"\") failed: %v", err)
	}

	// Verify it's not empty
	if repoDir == "" {
		t.Fatal("EnsureXMLUIRepo() returned empty directory")
	}

	t.Logf("Repository cached at: %s", repoDir)

	// Verify the directory exists
	info, err := os.Stat(repoDir)
	if err != nil {
		t.Fatalf("Repository directory does not exist: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("Repository path is not a directory")
	}

	// Verify essential directories exist
	essentialDirs := []string{
		"docs",
		"xmlui",
		filepath.Join("docs", "content", "components"),
		filepath.Join("docs", "public", "pages"),
		filepath.Join("xmlui", "src"),
	}

	for _, dir := range essentialDirs {
		fullPath := filepath.Join(repoDir, dir)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("Essential directory missing: %s", dir)
		}
	}

	// Verify directory name contains version
	dirName := filepath.Base(repoDir)
	if !strings.HasPrefix(dirName, "xmlui@") {
		t.Errorf("Directory name %s does not start with xmlui@", dirName)
	}

	t.Logf("Cached version (from dir name): %s", dirName)

	// Test that subsequent calls use the cached version (should be fast)
	repoDir2, err := EnsureXMLUIRepo("")
	if err != nil {
		t.Fatalf("Second EnsureXMLUIRepo(\"\") call failed: %v", err)
	}

	if repoDir2 != repoDir {
		t.Errorf("Expected same directory, got different: %s vs %s", repoDir, repoDir2)
	}
}

// TestServerWithAutoDownload tests creating a server with automatic repo download
func TestServerWithAutoDownload(t *testing.T) {
	// This test requires internet connectivity
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Create server config (repo will be auto-downloaded)
	config := ServerConfig{
		HTTPMode: false,
		Port:     "8080",
	}

	// Create server
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("Server is nil")
	}

	// Verify tools are loaded
	tools := server.GetTools()
	if len(tools) == 0 {
		t.Error("No tools loaded")
	}

	t.Logf("Loaded %d tools", len(tools))

	// Verify prompts are loaded
	prompts := server.GetPrompts()
	if len(prompts) == 0 {
		t.Error("No prompts loaded")
	}

	t.Logf("Loaded %d prompts", len(prompts))

	// Verify session manager exists
	sessionMgr := server.GetSessionManager()
	if sessionMgr == nil {
		t.Fatal("Session manager is nil")
	}
}

// TestCachePersistence tests that the cache persists across multiple calls
func TestCachePersistence(t *testing.T) {
	// This test requires internet connectivity
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// First call - may download
	dir1, err := EnsureXMLUIRepo("")
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call - should use cache
	dir2, err := EnsureXMLUIRepo("")
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Verify same directory
	if dir1 != dir2 {
		t.Errorf("Different directories returned: %s vs %s", dir1, dir2)
	}

	t.Logf("Cache persistence verified at %s", dir1)
}
