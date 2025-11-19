package xmluimcp

import (
	"context"
	"log/slog"
	"os"

	"github.com/mikeschinkel/go-cliutil"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/mcpcfg"
)

// RunCLI is the CLI entry point for the XMLUI MCP server.
// It parses options, loads configuration, and starts the server.
// This function calls os.Exit() on errors, making it suitable for CLI use.
//
// Exit codes:
//   - 1: Options parsing failure
//   - 2: Configuration loading failure
//   - 3: Configuration parsing failure
//   - 4: Known runtime error (not currently used, but available)
//   - 5: Unknown runtime error
func RunCLI() {
	var err error
	var logger *slog.Logger
	var mcpCfg *mcpcfg.RootConfigV1
	var cfgOpts *mcpcfg.Options
	var options *common.Options
	var config *common.Config

	// Parse command-line flags
	cfgOpts, err = mcpcfg.GetOptions()
	if err != nil {
		cliutil.Stderrf("Failed to parse options: %v\n", err)
		os.Exit(cliutil.ExitOptionsParseError)
	}

	// Convert raw options to typed options
	options, err = ParseOptions(cfgOpts)
	if err != nil {
		cliutil.Stderrf("Failed to parse options: %v\n", err)
		os.Exit(cliutil.ExitOptionsParseError)
	}

	writer := cliutil.NewWriter(&cliutil.WriterArgs{
		Quiet:     options.Quiet(),
		Verbosity: options.Verbosity(),
	})

	// TODO: Create logger (currently nil, add logging support later)
	logger = nil

	// Load root configuration
	mcpCfg, err = mcpcfg.LoadRootConfigV1(mcpcfg.LoadRootConfigV1Args{
		AppInfo: AppInfo(),
		Options: cfgOpts,
	})
	if err != nil {
		writer.Errorf("Failed to load config: %v\n", err)
		os.Exit(cliutil.ExitConfigLoadError)
	}

	// Parse configuration
	config, err = ParseConfig(mcpCfg, common.ConfigArgs{
		Options: options,
		AppInfo: AppInfo(),
		Logger:  logger,
		Writer:  writer,
	})
	if err != nil {
		writer.Errorf("Failed to parse config: %v\n", err)
		os.Exit(cliutil.ExitConfigParseError)
	}

	// Create context
	ctx := context.Background()

	// Create RunArgs
	args := &RunArgs{
		CLIArgs: os.Args[1:],
		Config:  config,
		Options: options,
	}

	// Run the server
	err = Run(ctx, args)

	if err != nil {
		writer.Errorf("MCP server error: %v\n", err)
		os.Exit(cliutil.ExitUnknownRuntimeError)
	}
}
