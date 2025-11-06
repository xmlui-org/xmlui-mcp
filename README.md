# xmlui-mcp: Model Context Protocol server for XMLUI

This project provides both a standalone CLI tool and a Go library for integrating XMLUI MCP capabilities into other applications.

This kit provides an MCP server that you can use with an MCP-aware tool, like Claude Desktop or Cursor, to empower those agents to help you build [XMLUI](https://xmlui.org) apps.

## Features

- **Standalone CLI**: Run as a command-line tool for MCP clients
- **Go Library**: Import into other Go applications as a library
- **Multiple Modes**: Support for stdio and HTTP server modes
- **Session Management**: Track and manage multiple user sessions
- **Analytics**: Built-in usage tracking and analytics
- **Extensible**: Easy to extend with custom tools and prompts

- [Prerequisites](#prerequisites)
- [Install](#install)
- [Configure](#configure)
- [Test](#test-the-server)
- [Library Usage](#library-usage)
- [API Reference](#api-reference)

## Prerequisites

The MCP server needs the [XMLUI repo](https://github.com/xmlui-org/xmlui) to exist as `$HOME/xmlui` (or `%USERPROFILE%\xmlui` on Windows). Clone that repo and make sure you have:

- `$HOME/xmlui/docs/content/components/` - Component documentation (.md files)
- `$HOME/xmlui/docs/public/pages/` - General documentation and tutorials
- `$HOME/xmlui/docs/public/pages/howto` - HowTo docs with working playgrounds
- `$HOME/xmlui/xmlui/src/components/` - Source code (.tsx, .scss files)

The MCP server will search these directories for component documentation, source code, and examples to help with XMLUI development.

## Install

To install, download the zip for your platform from [https://github.com/xmlui-org/xmlui-mcp/releases](https://github.com/xmlui-org/xmlui-mcp/releases), unzip, and cd into xmlui-mcp.

On Mac or Linux, run `prepare-binary.sh` to handle permissions.

## Configure

If you run the server interactively with no arguments it says:

```
Usage: ./xmlui-mcp [--http] [--port PORT] <xmluiDir> [exampleRoot] [comma-separated-exampleDirs]
  --http: Run in HTTP mode (default: stdio mode)
  --port: Port to listen on in HTTP mode (default: 8080)
```

- **./xmlui-mcp** The server binary

- **xmluiDir** The directory where the binary lives, along with source and doc files searched by the binary.

- **exampleRoot** An optional directory to search for examples

- **comma-separated-exampleDirs** Subdirectories under exampleRoot

Here's how that maps into a configuration for Claude Desktop, Cursor, or Copilot using the default stdio mode.

```
{
  "mcpServers": {
    "xmlui": {
      "command": "/Users/jonudell/xmlui-mcp/xmlui-mcp",
      "args": [
        "/Users/jonudell/xmlui-mcp/mcp",
        "/Users/jonudell",
        "xmlui-invoice,xmlui-mastodon"
      ]
    }
  }
}
```

The paths for these config files on a Mac are:

**Claude:** ~/Library/Application Support/Claude/claude_desktop_config.json

**Cursor:** ~/.cursor.mcp.json

**Copilot:** ~/Library/Application Support/Code/User/mcp.json (created initially by `MCP: Add Server`)

With this setup I am giving the agents access to the XMLUI projects I'm working on (xmlui-invoice, xmlui-mastodon), so they can both read and write those projects.

I am also encouraging them to use the xmlui-mcp tools as we work on those projects. Here's what that looks like.

<img width="737" alt="image" src="https://github.com/user-attachments/assets/1f87519c-1338-4eca-a730-9f2e0c1a64a9" />

<img width="788" alt="image" src="https://github.com/user-attachments/assets/4793a475-46d1-418e-ad6a-0760af53ddca" />

## Tips for working with agents that use this server

As agents use this server to search docs and examples, they receive strong guidance to prefer working examples, cite URLs when found, and admit ignorance when not found.

It helps to reinforce that guidance in their rules files.

- Obey the guidance you receive from the xmlui-mcp server.

- I will disbelieve any answer for which you cannot cite an URL to documentation or a working example.

- If you don't find an URL, say so.

- If you do find one, cite it.

Despite all this guidance, agents can wrongly report solutions for which they did not find documented examples. You can minimize that risk by being explicit in every interaction.

Instead of: _How can I right-align a Column in an XMLUI Table_

Say: _Show me a documented example of right-aligning a Column in an XMLUI Table_

## Library Usage

This project can be used as a Go library in other applications. The CLI is a thin wrapper around the library functionality.

### Basic Usage

```go
import "xmlui-mcp/pkg/xmlui"

func main() {
    config := xmlui.ServerConfig{
        XMLUIDir:    "/path/to/xmlui/source",
        ExampleRoot: "/path/to/examples",
        ExampleDirs: []string{"demo", "tutorial"},
        HTTPMode:    false, // stdio mode
        Port:        "8080",
    }

    server, err := xmlui.NewServer(config)
    if err != nil {
        log.Fatal(err)
    }

    // Print startup information
    server.PrintStartupInfo()

    // Start the server
    if config.HTTPMode {
        err = server.ServeHTTP()
    } else {
        err = server.ServeStdio()
    }

    if err != nil {
        log.Fatal(err)
    }
}
```

### Integration Example

```go
// In your go.mod:
// require xmlui-mcp v0.1.0

// Then import and use:
import "xmlui-mcp/pkg/xmlui"

type MyApp struct {
    mcpServer *xmlui.MCPServer
}

func NewMyApp(xmluiDir string) (*MyApp, error) {
    config := xmlui.ServerConfig{
        XMLUIDir: xmluiDir,
        HTTPMode: true,
        Port:     "8080",
    }

    server, err := xmlui.NewServer(config)
    if err != nil {
        return nil, err
    }

    return &MyApp{mcpServer: server}, nil
}

func (app *MyApp) Start() error {
    app.mcpServer.PrintStartupInfo()
    return app.mcpServer.ServeHTTP()
}
```

### Available Methods

- `NewServer(config ServerConfig) (*MCPServer, error)` - Create a new server instance
- `ServeStdio() error` - Start server in stdio mode
- `ServeHTTP() error` - Start server in HTTP mode
- `GetTools() []mcp.Tool` - Get available tools
- `GetPrompts() []mcp.Prompt` - Get available prompts
- `GetSessionManager() *SessionManager` - Get session manager
- `PrintStartupInfo()` - Print server information

### Configuration

The `ServerConfig` struct supports:

- `XMLUIDir` (required): Path to XMLUI source directory
- `ExampleRoot`: Root directory for examples
- `ExampleDirs`: Subdirectories within example root
- `HTTPMode`: Whether to run in HTTP mode
- `Port`: Port for HTTP mode (default: "8080")
- `AnalyticsFile`: Path to analytics file

## API Reference

### ServerConfig

Configuration for the XMLUI MCP server.

```go
type ServerConfig struct {
    XMLUIDir     string   // Path to XMLUI source directory
    ExampleRoot  string   // Optional: root directory for examples
    ExampleDirs  []string // Optional: subdirectories within example root
    HTTPMode     bool     // Whether to run in HTTP mode
    Port         string   // Port for HTTP mode (default: "8080")
    AnalyticsFile string  // Path to analytics file (optional)
}
```

### MCPServer

Main server instance with methods for starting and managing the server.

### SessionManager

Manages multiple user sessions with injected prompts and context.

## Test the server

Agents like Claude, Cursor, and Copilot typically use the server in stdio mode. You can use the server directly that way.

```
npx @modelcontextprotocol/inspector
http://localhost:6274/?MCP_PROXY_AUTH_TOKEN=550d43b6ac5b2881185e81c9c4abdb8115bc90ac3e277d97076bfcc84f4d6288
```

If a browser does not auto-launch, copy/paste that URL. In the inspector, fill in Command and Arguments.

Command: /path/to/xmlui-mcp

Arguments: /path/to/xmlui path/to/examples "folder1,folder2"

Then click Connect.

## Building from Source

### Library Build

To build just the library:

```bash
go build ./pkg/xmlui/...
```

### CLI Build

To build the CLI executable:

```bash
go build -o xmlui-mcp ./cmd/xmlui-mcp
```

### Development

The project structure is organized as:

```
xmlui-mcp/
├── pkg/xmlui/          # Library package
├── cmd/xmlui-mcp/      # CLI executable
├── server/             # Tool implementations
└── example/            # Library usage examples
```

## Analytics

The server saves logs to enable tracking agent usage patterns and optimizing the tools. Data is saved to `xmlui-mcp-analytics.json`. Use `./analytics-helper.sh` for overviews of what's been captured.
