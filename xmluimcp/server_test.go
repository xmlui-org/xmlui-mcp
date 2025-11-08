package xmluimcp

import (
	"os"
	"testing"

	"github.com/xmlui-org/mcp/xmluimcp/common"
)

func TestNewServer(t *testing.T) {
	config := common.ServerConfig{
		XMLUIDir:    "/tmp/test-xmlui",
		ExampleRoot: "/tmp/test-examples",
		ExampleDirs: []string{"demo"},
		HTTPMode:    false,
		Port:        "8080",
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Test that tools are initialized
	tools := server.GetTools()
	if len(tools) == 0 {
		t.Error("Server should have tools initialized")
	}

	// Test that prompts are initialized
	prompts := server.GetPrompts()
	if len(prompts) == 0 {
		t.Error("Server should have prompts initialized")
	}

	// Test session manager
	sessionManager := server.GetSessionManager()
	if sessionManager == nil {
		t.Error("Session manager should not be nil")
	}
}

func TestServerConfigValidation(t *testing.T) {
	// Test that empty XMLUIDir triggers auto-download (skip if network unavailable)
	if os.Getenv("SKIP_NETWORK_TESTS") == "" {
		config := common.ServerConfig{
			XMLUIDir: "",
		}

		server, err := NewServer(config)
		if err != nil {
			t.Logf("Auto-download failed (may be expected): %v", err)
		} else if server != nil {
			// Verify that XMLUIDir was populated
			if server.config.XMLUIDir == "" {
				t.Error("XMLUIDir should be populated after auto-download")
			}
			t.Logf("Auto-download successful, using: %s", server.config.XMLUIDir)
		}
	}

	// Test default port
	config := common.ServerConfig{
		XMLUIDir: "/tmp/test",
		Port:     "",
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server with empty port: %v", err)
	}

	if server.config.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", server.config.Port)
	}
}

func TestSessionManager(t *testing.T) {
	sessionManager := &SessionManager{
		sessions: make(map[string]*SessionContext),
	}

	// Test creating a new session
	session := sessionManager.GetOrCreateSession("test-session")
	if session == nil {
		t.Fatal("Session should not be nil")
	}

	if session.ID != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", session.ID)
	}

	// Test getting existing session
	session2 := sessionManager.GetOrCreateSession("test-session")
	if session2 != session {
		t.Error("Should return the same session instance")
	}

	// Test listing sessions
	sessions := sessionManager.ListSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0] != "test-session" {
		t.Errorf("Expected session 'test-session', got '%s'", sessions[0])
	}

	// Test removing session
	removed := sessionManager.RemoveSession("test-session")
	if !removed {
		t.Error("Should have removed session")
	}

	sessions = sessionManager.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after removal, got %d", len(sessions))
	}
}

func TestNewServerAutoDownload(t *testing.T) {
	// Skip if network tests are disabled
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network test")
	}

	// Test creating server with empty XMLUIDir (should trigger auto-download)
	config := common.ServerConfig{
		XMLUIDir: "",
		HTTPMode: false,
		Port:     "8080",
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server with auto-download: %v", err)
	}

	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Verify that XMLUIDir was populated
	if server.config.XMLUIDir == "" {
		t.Fatal("XMLUIDir should be populated after auto-download")
	}

	t.Logf("Auto-downloaded repository to: %s", server.config.XMLUIDir)

	// Verify tools are loaded
	tools := server.GetTools()
	if len(tools) == 0 {
		t.Error("Server should have tools initialized")
	}

	// Verify prompts are loaded
	prompts := server.GetPrompts()
	if len(prompts) == 0 {
		t.Error("Server should have prompts initialized")
	}
}
