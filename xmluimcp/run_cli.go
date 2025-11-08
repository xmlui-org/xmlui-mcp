package xmluimcp

import (
	"context"
	"fmt"
	"os"

	"github.com/xmlui-org/mcpsvr/xmluimcp/common"
)

// RunCLI is the CLI entry point for the XMLUI MCP server.
// It converts common.Options into ServerConfig and starts the server.
// This function calls os.Exit() on errors, making it suitable for CLI use.
//
// If opts is nil, it will use default configuration.
//
// Exit codes:
//   - 1: Server creation failure
//   - 2: Server execution error
func RunCLI(opts *common.Options) {
	var err error
	var config common.ServerConfig

	// Use defaults if opts is nil
	if opts == nil {
		opts = &common.Options{}
	}

	// Build ServerConfig from Options
	config = common.ServerConfig{
		XMLUIDir:      opts.XMLUIDir, // Empty means auto-download
		ExampleRoot:   opts.ExampleRoot,
		ExampleDirs:   opts.ExampleDirs,
		HTTPMode:      false, // Always stdio mode for MCP protocol
		Port:          "8080",
		AnalyticsFile: opts.AnalyticsFile,
	}

	// Create context
	ctx := context.Background()

	// Create RunArgs
	args := &RunArgs{
		CLIArgs: os.Args[1:],
		Config:  config,
		Options: opts,
	}

	// Run the server
	err = Run(ctx, args)

	if err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(2)
	}
}
