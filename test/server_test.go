package test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mikeschinkel/go-cfgstore"
	"github.com/mikeschinkel/go-dt"
	"github.com/mikeschinkel/go-testutil"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
)

var _ dt.FilepathGetter = (*bufferedJSONHandler)(nil)

type bufferedJSONHandler struct {
	slog.Handler
	filepath dt.Filepath
}

func (h bufferedJSONHandler) Filepath() dt.Filepath {
	return h.filepath
}

func getNewMCPServer(svrCfg *common.ServerConfig) (svr *xmluimcp.MCPServer, err error) {
	var configDir dt.DirPath

	appInfo := xmluimcp.AppInfo()
	// TODO: Update this to use a filefixture with same data instead of live data on
	//  developer's machine.
	configDir, err = cfgstore.CLIConfigDir(appInfo.ConfigSlug())
	if err != nil {
		goto end
	}

	svr = xmluimcp.NewServer(&common.Config{
		Options: nil, // TODO: Set this, maybe?
		AppInfo: xmluimcp.AppInfo(),
		Writer:  testutil.NewBufferedWriter(),
		Server:  svrCfg,
		Logger: slog.New(bufferedJSONHandler{
			Handler:  testutil.NewBufferedLogHandler(),
			filepath: dt.FilepathJoin3(configDir, appInfo.LogPath(), appInfo.LogFile()),
		}),
	})
	err = svr.Initialize()
end:
	return svr, err
}
func TestNewServer(t *testing.T) {
	server, err := getNewMCPServer(&common.ServerConfig{
		XMLUIDir:    filepath.Join(os.TempDir(), "test-xmlui"),
		ExampleRoot: "/tmp/test-examples",
		ExampleDirs: []string{"demo"},
		HTTPMode:    false,
		Port:        "8080",
	})
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
		server, err := getNewMCPServer(&common.ServerConfig{
			XMLUIDir: "",
		})
		if err != nil {
			// t.Logf("Auto-download failed (may be expected): %v", err)
		} else if server != nil {
			// Verify that XMLUIDir was populated
			if server.Config().XMLUIDir == "" {
				t.Error("XMLUIDir should be populated after auto-download")
			}
			// t.Logf("Auto-download successful, using: %s", server.Config().XMLUIDir)
		}
	}

	// Test default port
	server, err := getNewMCPServer(&common.ServerConfig{
		XMLUIDir: filepath.Join(os.TempDir(), "test"),
		Port:     "",
	})
	if err != nil {
		t.Fatalf("Failed to create server with empty port: %v", err)
	}

	if server.Config().Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", server.Config().Port)
	}
}

func TestSessionManager(t *testing.T) {
	sessionManager := xmluimcp.NewSessionManager()

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
