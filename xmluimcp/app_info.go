package xmluimcp

import (
	"github.com/mikeschinkel/go-dt/appinfo"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
)

var appInfo = appinfo.New(appinfo.Args{
	Name:        common.AppName,
	Description: common.AppDescr,
	Version:     common.AppVer,
	AppSlug:     common.AppSlug,
	ConfigSlug:  common.ConfigSlug,
	ConfigFile:  common.ConfigFile,
	InfoURL:     common.InfoURL,
	ExeName:     common.ExeName,
	LogFile:     common.LogFile,
	LogPath:     common.LogPath,
	ExtraInfo:   common.ExtraInfo,
})

func AppInfo() appinfo.AppInfo {
	return appInfo
}
