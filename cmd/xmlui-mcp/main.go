package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"xmlui-mcp/pkg/xmluimcp"
)

func main() {
	// Define command-line flags
	var (
		httpMode = flag.Bool("http", false, "Run in HTTP mode instead of stdio")
		port     = flag.String("port", "8080", "Port to listen on in HTTP mode")
	)

	// Parse flags but allow positional arguments
	flag.Parse()

	// Get positional arguments after flags
	args := flag.Args()

	xmluiDir := ""
	exampleRoot := ""
	exampleDirs := []string{}

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

	// Create server configuration
	config := xmluimcp.ServerConfig{
		XMLUIDir:    xmluiDir,
		ExampleRoot: exampleRoot,
		ExampleDirs: exampleDirs,
		HTTPMode:    *httpMode,
		Port:        *port,
	}

	// Create and start the server
	server, err := xmluimcp.NewServer(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Print startup information
	server.PrintStartupInfo()

	// Start server based on mode
	if config.HTTPMode {
		err = server.ServeHTTP()
	} else {
		err = server.ServeStdio()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
