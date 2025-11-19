package xmluimcp

import (
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpcfg"
)

type ParseConfigArgs = common.ConfigArgs

// ParseConfig converts raw config from mcpcfg.RootConfigV1 into
// typed common.Config with all runtime dependencies.
func ParseConfig(cfg *mcpcfg.RootConfigV1, args common.ConfigArgs) (config *common.Config, err error) {
	// Convert typed Options paths back to strings for ServerConfig
	var exampleDirs []string
	if args.Options != nil {
		exampleDirs = make([]string, len(args.Options.ExampleDirs))
		for i, dir := range args.Options.ExampleDirs {
			exampleDirs[i] = string(dir)
		}
	}

	serverConfig := &common.ServerConfig{
		XMLUIDir:      string(args.Options.XMLUIDir),
		ExampleRoot:   string(args.Options.ExampleRoot),
		ExampleDirs:   exampleDirs,
		HTTPMode:      cfg.HTTPMode,
		Port:          cfg.Port,
		AnalyticsFile: string(args.Options.AnalyticsFile),
	}

	config = common.NewConfig(common.ConfigArgs{
		Server:  serverConfig,
		Options: args.Options,
		AppInfo: args.AppInfo,
		Logger:  args.Logger,
		Writer:  args.Writer,
	})

	return config, err
}
