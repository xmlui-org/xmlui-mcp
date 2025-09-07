# xmlui-mcp: Model Context Protocol server for XMLUI

This kit provides an MCP server that you can use with an MCP-aware tool, like Claude Desktop or Cursor, to empower those agents to help you build [XMLUI](https://xmlui.org) apps.

- [Prerequisites](#prerequisites)
- [Install](#install)
- [Configure](#configure)
- [Test](#test-the-server)

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

# Guidance

As agents use this server to search docs and examples, they receive strong guidance to prefer working examples, cite URLs when found, and admit ignorance when not found. It helps to reinforce that guidance in their rules files.

- Obey the guidance you receive from the xmlui-mcp server.

- I will disbelieve any answer for which you cannot cite an URL to documentation or a working example.

- If you don't find an URL, say so.

- If you do find one, cite it.

## Analytics

The server saves logs to enable tracking agent usage patterns and optimizing the tools. Data is saved to `xmlui-mcp-analytics.json`. Use `./analytics-helper.sh` for overviews of what's been captured.

