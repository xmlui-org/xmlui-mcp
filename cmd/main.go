// Package main/cmd provides the entry point for the XMLUI MCP server application.
//
// The XMLUI MCP server provides Model Context Protocol (MCP) tools for working with
// the XMLUI framework documentation, examples, and components.
package main

import (
	"github.com/xmlui-org/xmlui-mcp/xmluimcp"
)

// main is the application entry point that starts the XMLUI MCP server.
func main() {
	xmluimcp.RunCLI()
}
