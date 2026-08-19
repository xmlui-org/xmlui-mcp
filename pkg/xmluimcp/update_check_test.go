package xmluimcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	mcpserver "xmlui-mcp/server"
)

func TestSemverLessThan(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "patch", a: "v0.0.9", b: "v0.0.10", want: true},
		{name: "equal", a: "v0.0.10", b: "0.0.10", want: false},
		{name: "greater", a: "0.1.0", b: "0.0.10", want: false},
		{name: "missing patch", a: "0.1", b: "0.1.1", want: true},
		{name: "prerelease suffix", a: "0.1.0-beta.1", b: "0.1.0", want: false},
		{name: "build suffix", a: "0.1.0+build.1", b: "0.1.1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semverLessThan(tt.a, tt.b); got != tt.want {
				t.Fatalf("semverLessThan(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func stubUpdateCheck(t *testing.T, fetch func() (string, error)) {
	t.Helper()
	prevFetch := fetchLatestCLITag
	updateStateMu.Lock()
	prevState := updateState
	updateStateMu.Unlock()
	fetchLatestCLITag = fetch
	t.Cleanup(func() {
		fetchLatestCLITag = prevFetch
		updateStateMu.Lock()
		updateState = prevState
		updateStateMu.Unlock()
	})
}

// The notice is an instruction to the assistant: it must carry the relay
// directive, the restart-sessions guidance, and the do-not-act boundary.
func TestUpdateNoticeCarriesRelayInstruction(t *testing.T) {
	stubUpdateCheck(t, func() (string, error) { return "v0.2.0", nil })

	notice := refreshUpdateState("v0.1.0")
	if notice == "" {
		t.Fatal("expected a notice for an available update")
	}
	for _, want := range []string{
		"for the assistant",
		"Tell your user",
		"restart any agent",
		"keep the old",
		"Relay this once per session",
		"Do not run the installer",
		"v0.1.0",
		"v0.2.0",
	} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice missing %q:\n%s", want, notice)
		}
	}
	if got := currentUpdateNotice(); got != notice {
		t.Fatal("currentUpdateNotice must reflect refreshed state")
	}
	status := currentUpdateStatus()
	if !status.Available || status.Latest != "v0.2.0" || status.Installed != "v0.1.0" {
		t.Fatalf("unexpected state: %+v", status)
	}
}

func TestUpdateCheckUpToDateAndDevAndFailure(t *testing.T) {
	stubUpdateCheck(t, func() (string, error) { return "v0.1.0", nil })
	if notice := refreshUpdateState("v0.1.0"); notice != "" {
		t.Fatalf("up-to-date must yield no notice, got %q", notice)
	}
	if currentUpdateNotice() != "" {
		t.Fatal("no notice may persist when up to date")
	}

	called := false
	stubUpdateCheck(t, func() (string, error) { called = true; return "v9.9.9", nil })
	if notice := refreshUpdateState("dev"); notice != "" || called {
		t.Fatalf("dev builds must skip the check entirely (notice=%q called=%v)", notice, called)
	}

	stubUpdateCheck(t, func() (string, error) { return "", errors.New("network down") })
	if notice := refreshUpdateState("v0.1.0"); notice != "" {
		t.Fatalf("check failure must yield no notice, got %q", notice)
	}
}

func TestBuildStatusText(t *testing.T) {
	stubUpdateCheck(t, func() (string, error) { return "v0.2.0", nil })
	refreshUpdateState("v0.1.0")

	status := buildStatusText("v0.1.0", "/cache/xmlui-repos/xmlui@0.14.9")
	for _, want := range []string{
		"Installed CLI version: v0.1.0",
		"v0.2.0 (update available)",
		"Docs corpus in use: xmlui@0.14.9",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status missing %q:\n%s", want, status)
		}
	}
	// The notice is NOT embedded: the analytics wrapper prepends the one
	// canonical copy; embedding doubled it in a single result (#31).
	if strings.Contains(status, "Tell your user") {
		t.Fatalf("status must not embed the update notice:\n%s", status)
	}
}

func TestUpdateNoticeVersionDisplayConsistent(t *testing.T) {
	notice := buildUpdateNotice("0.0.9", "v0.1.0")
	if !strings.Contains(notice, "v0.0.9 → v0.1.0") {
		t.Fatalf("mixed version display not normalized:\n%s", notice)
	}
}

// A wrapped status call must carry exactly one copy of the notice — the
// wrapper's prepend — not a second embedded one (#31).
func TestStatusResultCarriesSingleNoticeCopy(t *testing.T) {
	stubUpdateCheck(t, func() (string, error) { return "v0.2.0", nil })
	notice := refreshUpdateState("v0.1.0")
	if notice == "" {
		t.Fatal("expected notice")
	}
	mcpserver.SetUpdateNotice(notice)
	t.Cleanup(func() { mcpserver.SetUpdateNotice("") })

	handler := mcpserver.WithAnalytics("xmlui_status", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(buildStatusText("v0.1.0", "/cache/xmlui-repos/xmlui@0.14.9")), nil
	})
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if got := strings.Count(text, "Tell your user"); got != 1 {
		t.Fatalf("notice copies = %d, want exactly 1:\n%s", got, text)
	}
}
