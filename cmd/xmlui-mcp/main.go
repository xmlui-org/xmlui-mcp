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

	var xmluiDir string
	var exampleRoot string
	var exampleDirs []string

	// If no xmluiDir argument is provided, use the cached repository
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No XMLUI directory specified, using cached repository...")
		cachedRepo, err := xmluimcp.EnsureXMLUIRepo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error ensuring XMLUI repository: %v\n", err)
			fmt.Fprintln(os.Stderr, "\nAlternatively, you can specify a local XMLUI directory:")
			fmt.Fprintln(os.Stderr, "Usage: ./xmlui-mcp [--http] [--port PORT] <xmluiDir> [exampleRoot] [comma-separated-exampleDirs]")
			fmt.Fprintln(os.Stderr, "  --http: Run in HTTP mode (default: stdio mode)")
			fmt.Fprintln(os.Stderr, "  --port: Port to listen on in HTTP mode (default: 8080)")
			os.Exit(1)
		}
		xmluiDir = cachedRepo
		exampleRoot = ""
		exampleDirs = []string{}
	} else {
		// Use provided arguments
		xmluiDir = args[0]

		// Optional arg 2: example root
		if len(args) >= 2 {
			exampleRoot = args[1]
		}

		// Optional arg 3: comma-separated subdirs
		if len(args) >= 3 {
			exampleDirs = strings.Split(args[2], ",")
		}
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
