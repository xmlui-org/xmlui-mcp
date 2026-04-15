package xmluimcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcpserver "xmlui-mcp/server"
)

const latestCLIReleaseURL = "https://api.github.com/repos/xmlui-org/xmlui-cli/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// checkForUpdate compares the running CLI version against the latest GitHub release.
// It returns a notice if an update is available, or an empty string otherwise.
func checkForUpdate(currentVersion string) string {
	currentVersion = strings.TrimSpace(currentVersion)
	if currentVersion == "" || currentVersion == "dev" {
		return ""
	}

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, latestCLIReleaseURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "xmlui-cli/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		mcpserver.WriteDebugLog("Update check failed: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		mcpserver.WriteDebugLog("Update check returned status %d\n", resp.StatusCode)
		return ""
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		mcpserver.WriteDebugLog("Update check failed to decode response: %v\n", err)
		return ""
	}

	latest := strings.TrimSpace(release.TagName)
	if latest == "" {
		return ""
	}

	if semverLessThan(currentVersion, latest) {
		return fmt.Sprintf(
			"A newer XMLUI CLI is available. Installed: %s. Latest: %s. "+
				"Upgrading xmlui-cli also updates the bundled xmlui-mcp. "+
				"Download: https://github.com/xmlui-org/xmlui-cli/releases/latest",
			currentVersion, latest)
	}

	return ""
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
