package common

type Config = ServerConfig

// ServerConfig holds configuration for the XMLUI MCP server
type ServerConfig struct {
	XMLUIDir      string   // Path to XMLUI source directory
	ExampleRoot   string   // Optional: root directory for examples
	ExampleDirs   []string // Optional: subdirectories within example root
	HTTPMode      bool     // Whether to run in HTTP mode
	Port          string   // Port for HTTP mode (default: "8080")
	AnalyticsFile string   // Path to analytics file (optional)
}

// Config implements cliutil.Config interface
func (sc ServerConfig) Config() {}
