# xmlui-mcp: Model Context Protocol server for XMLUI

This project provides both a standalone CLI tool and a Go library for integrating XMLUI MCP capabilities into other applications.

This kit provides an MCP server that you can use with an MCP-aware tool, like Claude Desktop or Cursor, to empower those agents to help you build [XMLUI](https://xmlui.org) apps.

## Features

- **Standalone CLI**: Run as a command-line tool for MCP clients
- **Go Library**: Import into other Go applications as a library (in development, API isn't stable)
- **Multiple Modes**: Support for stdio and HTTP server modes
- **Session Management**: Track and manage multiple user sessions
- **Analytics**: Built-in usage tracking and analytics
- **Extensible**: Easy to extend with custom tools and prompts

## Sections

- [Install](#install)
- [Configure](#configure)
- [Usage](#usage)
- [Test](#test-the-server)
- [Analytics](#analytics)

The download happens once and is reused on subsequent runs. The server will fetch the latest `xmlui@<version>` release from GitHub.

## Install

To install, download the zip for your platform from [https://github.com/xmlui-org/xmlui-mcp/releases](https://github.com/xmlui-org/xmlui-mcp/releases) and unzip into a known location like ~/xmlui-mcp. On Mac, run `prepare-binary.sh` to handle permissions.

## Configure

The simplest configuration just names the location of the binary. The server will search the cached repo.

```json
{
  "mcpServers": {
    "xmlui": {
      "command": "/Users/jonudell/xmlui-mcp/xmlui-mcp"
    }
  }
}
```

- With the `-e` or `--example` options, additional XMLUI projects will be searched.
- With the `--version` option, you can choose a specific xmlui version to use

```json
{
  "mcpServers": {
    "xmlui": {
      "command": "/Users/jonudell/xmlui-mcp/xmlui-mcp",
      "args": [
        "--example",
        "/Users/jonudell/xmlui-invoice",
        "--example",
        "/Users/jonudell/xmlui-weather",
        "--xmlui-version",
        "0.11.4"
      ]
    }
  }
}
```

Note: The syntax can (annoyingly) vary by coding assistant. With Codex, for example, it is `.toml` not `.json` and looks like this:

```toml
[mcp_servers.xmlui]
command = "/Users/jonudell/xmlui-mcp/xmlui-mcp"
args = [
    "--example",
    "/Users/jonudell/xmlui-invoice",
    "--example",
    "/Users/jonudell/xmlui-weather",
    "--xmlui-version",
    "0.11.4"
]
```

## Usage

```
Usage of xmlui-mcp:
  -e value
        Example directory path (can be repeated)
  -example value
        Example directory path (can be repeated, alias for -e)
  -http
        Run in HTTP mode instead of stdio
  -port string
        Port to listen on in HTTP mode (default "8080")
  -xmlui-version string
        Specific XMLUI version to use (e.g. 0.11.4)
```

- **./xmlui-mcp** The server binary

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

## Test the server

Agents like Claude, Cursor, and Copilot typically use the server in stdio mode. You can use the server directly that way.

```
npx @modelcontextprotocol/inspector
http://localhost:6274/?MCP_PROXY_AUTH_TOKEN=550d43b6ac5b2881185e81c9c4abdb8115bc90ac3e277d97076bfcc84f4d6288
```

If a browser does not auto-launch, copy/paste that URL. In the inspector, fill in Command and Arguments.

Command: /path/to/xmlui-mcp
Arguments: -e path/to/examples

Then click Connect.

## Analytics

In order to be able to search XMLUI documentation and source code, the server will automatically download the specified (or latest) XMLUI repository from GitHub. The server saves logs to enable tracking agent usage patterns and optimizing the tools. Data is saved to a file named `xmlui-mcp-analytics.json`. Use `./analytics-helper.sh` for overviews of what's been captured. The saved files are located at:

- **Linux**: `$XDG_CACHE_HOME/xmlui/xmlui-mcp` or `~/.cache/xmlui/xmlui-mcp`
- **macOS**: `~/Library/Caches/xmlui/xmlui-mcp`
- **Windows**: `%LOCALAPPDATA%\xmlui\xmlui-mcp`
