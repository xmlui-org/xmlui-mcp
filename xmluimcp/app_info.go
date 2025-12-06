package xmluimcp

import (
	"github.com/mikeschinkel/go-dt/appinfo"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpsvr"
)

var appInfo = appinfo.New(appinfo.Args{
	Name:        mcpsvr.AppName,
	Description: mcpsvr.AppDescr,
	Version:     mcpsvr.AppVer,
	AppSlug:     mcpsvr.AppSlug,
	ConfigSlug:  mcpsvr.ConfigSlug,
	ConfigFile:  mcpsvr.ConfigFile,
	InfoURL:     mcpsvr.InfoURL,
	ExeName:     mcpsvr.ExeName,
	LogFile:     mcpsvr.LogFile,
	LogPath:     mcpsvr.LogPath,
	ExtraInfo:   mcpsvr.ExtraInfo,
})

func AppInfo() appinfo.AppInfo {
	return appInfo
}
