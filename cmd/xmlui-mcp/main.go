package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"xmlui-mcp/pkg/xmluimcp"
)

// stringSlice handles repeated string flags
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	// Define command-line flags
	var (
		httpMode     = flag.Bool("http", false, "Run in HTTP mode instead of stdio")
		port         = flag.String("port", "8080", "Port to listen on in HTTP mode")
		xmluiVersion = flag.String("xmlui-version", "", "Specific XMLUI version to use (e.g. 0.11.4)")
		exampleDirs  stringSlice
	)

	// Bind example flag and its alias
	flag.Var(&exampleDirs, "example", "Example directory path (can be repeated, alias for -e)")
	flag.Var(&exampleDirs, "e", "Example directory path (can be repeated)")

	// Parse flags
	flag.Parse()

	// Create server configuration
	// The XMLUI repository will be automatically downloaded and cached by NewServer
	config := xmluimcp.ServerConfig{
		ExampleDirs:  exampleDirs,
		HTTPMode:     *httpMode,
		Port:         *port,
		XMLUIVersion: *xmluiVersion,
	}

	// Create and start the server
	server, err := xmluimcp.NewServer(config)
	if err != nil {
		if errors.Is(err, xmluimcp.ErrVersionNotFound) && *xmluiVersion != "" {
			fmt.Fprintf(os.Stderr, "Error: The specified XMLUI version '%s' was not found.\nPlease check if it is a valid version.\n", *xmluiVersion)
		} else {
			fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		}
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
