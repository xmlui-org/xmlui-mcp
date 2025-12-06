package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mikeschinkel/go-testutil"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpsvr"
)

// TestEnsureXMLUIRepoIntegration tests the full integration of automatic repo download
func TestEnsureXMLUIRepoIntegration(t *testing.T) {
	// This test requires internet connectivity
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Get the repo directory
	repoDir, err := xmluimcp.EnsureXMLUIRepo(testutil.GetBufferedLogger())
	if err != nil {
		t.Fatalf("EnsureXMLUIRepo() failed: %v", err)
	}

	// Verify it's not empty
	if repoDir == "" {
		t.Fatal("EnsureXMLUIRepo() returned empty directory")
	}

	// t.Logf("Repository cached at: %s", repoDir)

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

	// Verify version marker exists
	version, err := xmluimcp.ReadVersionMarker(repoDir)
	if err != nil {
		t.Fatalf("Version marker not found: %v", err)
	}

	// t.Logf("Cached version: %s", version)

	// Verify version format
	if len(version) < 6 || version[:6] != "xmlui@" {
		t.Errorf("Invalid version format: %s", version)
	}

	// Test that subsequent calls use the cached version (should be fast)
	repoDir2, err := xmluimcp.EnsureXMLUIRepo(testutil.GetBufferedLogger())
	if err != nil {
		t.Fatalf("Second EnsureXMLUIRepo() call failed: %v", err)
	}

	if repoDir2 != repoDir {
		t.Errorf("Expected same directory, got different: %s vs %s", repoDir, repoDir2)
	}
}

// TestServerWithDownloadedRepo tests creating a server with a manually downloaded repo
func TestServerWithDownloadedRepo(t *testing.T) {
	// This test requires internet connectivity
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Manually ensure repo is downloaded first
	xmluiDir, err := xmluimcp.EnsureXMLUIRepo(testutil.GetBufferedLogger())
	if err != nil {
		t.Fatalf("Failed to ensure XMLUI repo: %v", err)
	}

	// Create server
	server, err := getNewMCPServer(&mcpsvr.ServerConfig{
		XMLUIDir: xmluiDir,
		HTTPMode: false,
		Port:     "8080",
	})
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

	// t.Logf("Loaded %d tools", len(tools))

	// Verify prompts are loaded
	prompts := server.GetPrompts()
	if len(prompts) == 0 {
		t.Error("No prompts loaded")
	}

	// t.Logf("Loaded %d prompts", len(prompts))

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
	dir1, err := xmluimcp.EnsureXMLUIRepo(testutil.GetBufferedLogger())
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	version1, err := xmluimcp.ReadVersionMarker(dir1)
	if err != nil {
		t.Fatalf("Failed to read version marker: %v", err)
	}

	// Second call - should use cache
	dir2, err := xmluimcp.EnsureXMLUIRepo(testutil.GetBufferedLogger())
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	version2, err := xmluimcp.ReadVersionMarker(dir2)
	if err != nil {
		t.Fatalf("Failed to read version marker on second call: %v", err)
	}

	// Verify same directory and version
	if dir1 != dir2 {
		t.Errorf("Different directories returned: %s vs %s", dir1, dir2)
	}

	if version1 != version2 {
		t.Errorf("Different versions: %s vs %s", version1, version2)
	}

	// t.Logf("Cache persistence verified: %s at %s", version1, dir1)
}
