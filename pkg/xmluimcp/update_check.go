package xmluimcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	mcpserver "xmlui-mcp/server"
)

const latestCLIReleaseURL = "https://api.github.com/repos/xmlui-org/xmlui-cli/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// fetchLatestCLITag is injectable for tests; production value queries GitHub.
var fetchLatestCLITag = fetchLatestCLITagFromGitHub

// updateStatus is the last known result of an update check, served by the
// xmlui_status tool and used to build the agent-directed notice.
type updateStatus struct {
	Installed string
	Latest    string
	Available bool
	CheckedAt time.Time
}

var (
	updateStateMu sync.Mutex
	updateState   updateStatus
)

func currentUpdateStatus() updateStatus {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()
	return updateState
}

// currentUpdateNotice returns the agent-directed notice, or "" when no update
// is known to be available.
func currentUpdateNotice() string {
	status := currentUpdateStatus()
	if !status.Available {
		return ""
	}
	return buildUpdateNotice(status.Installed, status.Latest)
}

// buildUpdateNotice produces the notice delivered over every channel. It is
// addressed to the assistant, carries the exact sentence to relay to the user
// — including that running agent sessions keep the old server until restarted
// — and forbids acting on the user's behalf (prompt-injection hygiene: a tool
// result must never read as an instruction to execute installers).
func buildUpdateNotice(installed, latest string) string {
	return fmt.Sprintf(
		"[xmlui-mcp update notice — for the assistant] A newer XMLUI CLI (%s) is available; "+
			"this session is served by %s, which bundles an older xmlui-mcp server. "+
			"Tell your user at the end of your current reply: \"An XMLUI CLI update is available (%s → %s). "+
			"Reinstall from https://github.com/xmlui-org/xmlui-cli/releases/latest, then restart any agent "+
			"sessions that use the XMLUI MCP server — running sessions (including this one) keep the old "+
			"server until they are restarted.\" "+
			"Do not run the installer or any other command on the user's behalf; relay the message and continue with the task.",
		latest, installed, installed, latest)
}

func fetchLatestCLITagFromGitHub() (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, latestCLIReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "xmlui-cli")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update check returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return strings.TrimSpace(release.TagName), nil
}

// refreshUpdateState runs one update check, records the result, and returns
// the agent-directed notice ("" when up to date, unversioned, or on error).
func refreshUpdateState(currentVersion string) string {
	currentVersion = strings.TrimSpace(currentVersion)
	if currentVersion == "" || currentVersion == "dev" {
		return ""
	}

	latest, err := fetchLatestCLITag()
	if err != nil {
		mcpserver.WriteDebugLog("Update check failed: %v\n", err)
		return ""
	}
	if latest == "" {
		return ""
	}

	available := semverLessThan(currentVersion, latest)
	updateStateMu.Lock()
	updateState = updateStatus{
		Installed: currentVersion,
		Latest:    latest,
		Available: available,
		CheckedAt: time.Now(),
	}
	updateStateMu.Unlock()

	if !available {
		return ""
	}
	return buildUpdateNotice(currentVersion, latest)
}

// startPeriodicUpdateCheck re-runs the check on an interval so long-lived
// sessions learn about releases cut after startup. A newly found update
// re-arms the tool-result prepend channel; SetUpdateNotice with unchanged
// text is a no-op on the re-arm counter.
func startPeriodicUpdateCheck(currentVersion string, interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if notice := refreshUpdateState(currentVersion); notice != "" {
				mcpserver.SetUpdateNotice(notice)
			}
		}
	}()
}

// semverLessThan returns true if a < b using numeric semver comparison.
func semverLessThan(a, b string) bool {
	partsA := strings.Split(normalizeSemver(a), ".")
	partsB := strings.Split(normalizeSemver(b), ".")
	for i := 0; i < len(partsA) || i < len(partsB); i++ {
		var na, nb int
		if i < len(partsA) {
			na, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			nb, _ = strconv.Atoi(partsB[i])
		}
		if na < nb {
			return true
		}
		if na > nb {
			return false
		}
	}
	return false
}

func normalizeSemver(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if before, _, found := strings.Cut(version, "-"); found {
		version = before
	}
	if before, _, found := strings.Cut(version, "+"); found {
		version = before
	}
	return version
}

// buildStatusText renders the xmlui_status tool body from the current update
// state plus the running server's configuration.
func buildStatusText(cliVersion, xmluiDir string) string {
	var out strings.Builder
	out.WriteString("XMLUI MCP status\n")

	installed := strings.TrimSpace(cliVersion)
	if installed == "" {
		installed = "unknown"
	}
	fmt.Fprintf(&out, "- Installed CLI version: %s\n", installed)

	status := currentUpdateStatus()
	switch {
	case status.CheckedAt.IsZero():
		out.WriteString("- Latest CLI release: unknown (no successful update check yet)\n")
	case status.Available:
		fmt.Fprintf(&out, "- Latest CLI release: %s (update available)\n", status.Latest)
	default:
		fmt.Fprintf(&out, "- Latest CLI release: %s (up to date)\n", status.Latest)
	}

	fmt.Fprintf(&out, "- Docs corpus in use: %s\n", filepath.Base(xmluiDir))

	if notice := currentUpdateNotice(); notice != "" {
		out.WriteString("\n" + notice + "\n")
	}

	return out.String()
}
