package xmluimcp

import (
	"context"
)

// Run starts the XMLUI MCP server with the provided configuration.
// This is the main server execution function that creates the server instance
// and starts it in the appropriate mode (stdio or HTTP).
//
// The function follows this execution flow:
//  1. Create MCP server with provided configuration
//  2. Print startup information (tools, prompts, etc.)
//  3. Start server in stdio mode (default) or HTTP mode
//
// Returns an error if server creation or execution fails.
// This function blocks until the server terminates or encounters an error.
func Run(_ context.Context, args *RunArgs) error {
	// Create server
	server := NewServer(args.Config)
	err := server.Initialize()
	if err != nil {
		goto end
	}

	// Print startup information
	server.PrintStartupInfo()

	// Start server based on mode
	if args.Config.Server.HTTPMode {
		err = server.ServeHTTP()
		goto end
	}

	err = server.ServeStdio()

end:
	return err
}
