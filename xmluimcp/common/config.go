package common

import (
	"log/slog"

	"github.com/mikeschinkel/go-cliutil"
	"github.com/mikeschinkel/go-dt/appinfo"
)

// Config holds typed runtime configuration for the XMLUI MCP server
type Config struct {
	Server  *ServerConfig
	Options *Options
	AppInfo appinfo.AppInfo
	Logger  *slog.Logger
	Writer  cliutil.Writer
}

func (c *Config) Config() {}

// ServerConfig holds runtime server settings for the MCP server.
// It uses string paths (not typed) because the underlying MCP server expects strings.
type ServerConfig struct {
	XMLUIDir      string   // Path to XMLUI source directory
	ExampleRoot   string   // Optional: root directory for examples
	ExampleDirs   []string // Optional: subdirectories within example root
	HTTPMode      bool     // Whether to run in HTTP mode
	Port          string   // Port for HTTP mode (default: "8080")
	AnalyticsFile string   // Path to analytics file (optional)
}

type ConfigArgs struct {
	Server  *ServerConfig
	Options *Options
	AppInfo appinfo.AppInfo
	Logger  *slog.Logger
	Writer  cliutil.Writer
}

func NewConfig(args ConfigArgs) *Config {
	return &Config{
		Server:  args.Server,
		Options: args.Options,
		AppInfo: args.AppInfo,
		Logger:  args.Logger,
		Writer:  args.Writer,
	}
}
