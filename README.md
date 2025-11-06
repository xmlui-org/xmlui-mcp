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

### Automatic Mode (Recommended)

**No prerequisites needed!** The server will automatically download the latest XMLUI repository from GitHub if no directory is specified. The repository is cached in:

- **Linux**: `$XDG_CACHE_HOME/xmlui/xmlui-mcp` or `~/.cache/xmlui/xmlui-mcp`
- **macOS**: `~/Library/Caches/xmlui/xmlui-mcp`
- **Windows**: `%LOCALAPPDATA%\xmlui\xmlui-mcp`

The download happens once and is reused on subsequent runs. The server will fetch the latest `xmlui@<version>` release from GitHub.

### Manual Mode (Optional)

If you prefer to use a local XMLUI repository, clone the [XMLUI repo](https://github.com/xmlui-org/xmlui) and make sure you have:

- `docs/content/components/` - Component documentation (.md files)
- `docs/public/pages/` - General documentation and tutorials
- `docs/public/pages/howto` - HowTo docs with working playgrounds
- `xmlui/src/components/` - Source code (.tsx, .scss files)

The MCP server will search these directories for component documentation, source code, and examples to help with XMLUI development.

## Install

To install, download the zip for your platform from [https://github.com/xmlui-org/xmlui-mcp/releases](https://github.com/xmlui-org/xmlui-mcp/releases), unzip, and cd into xmlui-mcp.

On Mac or Linux, run `prepare-binary.sh` to handle permissions.

## Configure

### Automatic Configuration (Recommended)

The simplest configuration uses automatic repository download:

```json
{
  "mcpServers": {
    "xmlui": {
      "command": "/Users/jonudell/xmlui-mcp/xmlui-mcp"
    }
  }
}
```

On first run, the server will download the latest XMLUI repository from GitHub and cache it. No additional arguments needed!

### Manual Configuration (Optional)

If you want to use a local XMLUI directory or include custom examples:

```
Usage: ./xmlui-mcp [--http] [--port PORT] [xmluiDir] [exampleRoot] [comma-separated-exampleDirs]
  --http: Run in HTTP mode (default: stdio mode)
  --port: Port to listen on in HTTP mode (default: 8080)
```

- **./xmlui-mcp** The server binary

- **xmluiDir** (optional) The directory containing XMLUI source and docs. If omitted, the latest version is downloaded automatically.

- **exampleRoot** An optional directory to search for examples

- **comma-separated-exampleDirs** Subdirectories under exampleRoot

Example configuration with custom paths:

```json
{
  "mcpServers": {
    "xmlui": {
      "command": "/Users/jonudell/xmlui-mcp/xmlui-mcp",
      "args": [
        "/Users/jonudell/xmlui",
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

### Basic Usage with Automatic Download

```go
import "xmlui-mcp/pkg/xmluimcp"

func main() {
    // Ensure XMLUI repository is available (downloads if needed)
    xmluiDir, err := xmluimcp.EnsureXMLUIRepo()
    if err != nil {
        log.Fatal(err)
    }

    config := xmluimcp.ServerConfig{
        XMLUIDir:    xmluiDir,
        ExampleRoot: "/path/to/examples",
        ExampleDirs: []string{"demo", "tutorial"},
        HTTPMode:    false, // stdio mode
        Port:        "8080",
    }

    server, err := xmluimcp.NewServer(config)
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

### Manual Path Configuration

```go
import "xmlui-mcp/pkg/xmluimcp"

func main() {
    config := xmluimcp.ServerConfig{
        XMLUIDir:    "/path/to/xmlui/source",
        ExampleRoot: "/path/to/examples",
        ExampleDirs: []string{"demo", "tutorial"},
        HTTPMode:    false, // stdio mode
        Port:        "8080",
    }

    server, err := xmluimcp.NewServer(config)
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
import "xmlui-mcp/pkg/xmluimcp"

type MyApp struct {
    mcpServer *xmluimcp.MCPServer
}

func NewMyApp() (*MyApp, error) {
    // Automatically download and cache XMLUI repository
    xmluiDir, err := xmluimcp.EnsureXMLUIRepo()
    if err != nil {
        return nil, err
    }

    config := xmluimcp.ServerConfig{
        XMLUIDir: xmluiDir,
        HTTPMode: true,
        Port:     "8080",
    }

    server, err := xmluimcp.NewServer(config)
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

- `EnsureXMLUIRepo() (string, error)` - Download and cache XMLUI repository if needed, returns path
- `GetCacheDir() (string, error)` - Get platform-specific cache directory
- `GetRepoDir() (string, error)` - Get cached repository directory path
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
go build ./pkg/xmluimcp/...
```

### CLI Build

To build the CLI executable:

```bash
go build -o xmlui-mcp ./cmd/xmlui-mcp
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/xmluimcp -v

# Skip network tests (if offline)
SKIP_NETWORK_TESTS=1 go test ./pkg/xmluimcp -v
```

### Development

The project structure is organized as:

```
xmlui-mcp/
├── pkg/xmluimcp/       # Library package
│   ├── cache.go        # Platform-specific cache directory resolution
│   ├── repo_downloader.go  # GitHub repository download and caching
│   ├── server.go       # MCP server implementation
│   ├── session.go      # Session management
│   └── utils.go        # Utility functions
├── cmd/xmlui-mcp/      # CLI executable
├── server/             # Tool implementations
└── example/            # Library usage examples
```

### How Repository Caching Works

1. **First Run**: When started without a directory argument, the server queries GitHub for the latest `xmlui@<version>` release
2. **Download**: The repository is downloaded as a ZIP file to a temporary directory
3. **Extraction**: The ZIP is extracted and validated
4. **Atomic Move**: Once complete, the repository is atomically moved to the cache directory
5. **Subsequent Runs**: The cached version is used, no re-download needed

The atomic download process ensures that interrupted downloads or crashes don't leave the cache in a broken state.

## Analytics

The server saves logs to enable tracking agent usage patterns and optimizing the tools. Data is saved to `xmlui-mcp-analytics.json`. Use `./analytics-helper.sh` for overviews of what's been captured.
