package main

import (
	"flag"
	"os"
	"strings"

	"github.com/mikeschinkel/go-cliutil"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/common"
	"github.com/xmlui-org/xmlui-mcp/xmluimcp/xmluihelpers"
)

func main() {
	var xmluiDir string
	var exampleRoot string
	var exampleDirs = make([]string, 0)

	// Define command-line flags
	var (
		httpMode = flag.Bool("http", false, "Run in HTTP mode instead of stdio")
		port     = flag.String("port", "8080", "Port to listen on in HTTP mode")
	)

	// Parse flags but allow positional arguments
	flag.Parse()

	// Get positional arguments after flags
	args := flag.Args()

	// Optional arg 1: xmluiDir (if not provided, server will auto-download)
	if len(args) >= 1 {
		xmluiDir = args[0]
	}

	// Optional arg 2: example root
	if len(args) >= 2 {
		exampleRoot = args[1]
	}

	// Optional arg 3: comma-separated subdirs
	if len(args) >= 3 {
		exampleDirs = strings.Split(args[2], ",")
	}
	wr, err := xmluihelpers.CreateWriterLogger(&xmluihelpers.WriterLoggerArgs{
		Quiet:      false,
		Verbosity:  1,
		ConfigSlug: common.ConfigSlug,
		LogFile:    common.LogFile,
	})
	if err != nil {
		fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(cliutil.ExitLoggerSetupError)
	}

	// Create server configuration
	config := &common.Config{
		Server: &common.ServerConfig{
			XMLUIDir:      xmluiDir,
			ExampleRoot:   exampleRoot,
			ExampleDirs:   exampleDirs,
			HTTPMode:      *httpMode,
			Port:          *port,
			AnalyticsFile: "",
		},
		Options: nil, // TODO Implement this better
		AppInfo: xmluimcp.AppInfo(),
		Logger:  wr.Logger,
		Writer:  wr.Writer,
	}

	// Create and start the server
	server := xmluimcp.NewServer(config)
	err = server.Initialize()
	if err != nil {
		fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(cliutil.ExitConfigLoadError)
	}

	// Print startup information
	server.PrintStartupInfo()

	// Start server based on mode
	if config.Server.HTTPMode {
		err = server.ServeHTTP()
	} else {
		err = server.ServeStdio()
	}

	if err != nil {
		fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(cliutil.ExitUnknownRuntimeError)
	}
}
