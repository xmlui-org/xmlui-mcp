package mcpsvr

import (
	"github.com/mikeschinkel/go-dt"
)

// AppInfo consts
const (
	AppVer dt.Version = "v0.0.0" // TODO To be changed soon

	// AppName is the human-readable name of the application.
	AppName                 = "XMLUI MCP"
	AppDescr                = "XMLUI MCP Server"
	AppSlug  dt.PathSegment = "xmlui-mcp"

	// ConfigSlug provides the directory under ~/.config/ where configuration will be
	// stored. This is not xmlui-mcp as everything XMLUI goes under the one location.
	ConfigSlug dt.PathSegment = "xmlui"

	// ConfigFile is the path for where the config file will be stored in the config
	// directory, e.g. ~/.config/xmlui/mcpsvr.json
	ConfigFile dt.RelFilepath = "mcpsvr.json"

	// ExeName is the standalone name for this app when compiled as a standalone.
	// HOWEVER, the `xmlui` CLI should really be the only executable we put on a
	// user's machine; everything else gets loaded by the one CLI executable. We
	// are merely enabling this app to be separately compiled into an executable
	// for our own convenince andwe do not expect to distribute it.
	ExeName dt.Filename = "xmlui-mcp"

	LogPath dt.PathSegments = "logs"

	// GitHubRepoURL provides the GitHub repo for this project for use in error messages
	// TODO: Can we change this Github URL to be "https://github.com/xmlui-org/xmlui-mcp"?
	GitHubRepoURL dt.URL = "https://github.com/xmlui-org/mcp"
)

// Derived AppInfo consts
const (
	// InfoURL is Just a URL to display to users "for more information"
	InfoURL             = GitHubRepoURL
	LogFile dt.Filename = dt.Filename(string(AppSlug) + ".log")
)

var (
	ExtraInfo = map[string]any{
		"github_repo_url": GitHubRepoURL,
	}
)
