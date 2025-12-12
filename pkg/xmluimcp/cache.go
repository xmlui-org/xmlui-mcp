package xmluimcp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetCacheDir returns the platform-specific cache directory for XMLUI MCP
// - Linux: $XDG_CACHE_HOME/xmlui/xmlui-mcp or ~/.cache/xmlui/xmlui-mcp
// - macOS: ~/Library/Caches/xmlui/xmlui-mcp
// - Windows: %LOCALAPPDATA%\xmlui\xmlui-mcp
func GetCacheDir() (string, error) {
	var baseDir string
	var err error

	switch runtime.GOOS {
	case "windows":
		// Use %LOCALAPPDATA% on Windows
		baseDir = os.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			// Fallback to %USERPROFILE%\AppData\Local
			userProfile := os.Getenv("USERPROFILE")
			if userProfile == "" {
				return "", fmt.Errorf("cannot determine cache directory: LOCALAPPDATA and USERPROFILE not set")
			}
			baseDir = filepath.Join(userProfile, "AppData", "Local")
		}
		baseDir = filepath.Join(baseDir, "xmlui", "xmlui-mcp")

	case "darwin":
		// Use ~/Library/Caches on macOS
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, "Library", "Caches", "xmlui", "xmlui-mcp")

	default:
		// Linux and other Unix-like systems
		// Use $XDG_CACHE_HOME or ~/.cache
		baseDir = os.Getenv("XDG_CACHE_HOME")
		if baseDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			baseDir = filepath.Join(homeDir, ".cache")
		}
		baseDir = filepath.Join(baseDir, "xmlui", "xmlui-mcp")
	}

	// Ensure the directory exists
	if err = os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return baseDir, nil
}

// GetRepoDir returns the path where the XMLUI repository should be cached
func GetRepoDir() (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "repo"), nil
}

// GetReposDir returns the directory where XMLUI repositories are stored
func GetReposDir() (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "xmlui-repos"), nil
}
