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
   - `$HOME/xmlui/xmlui/src/components/` - Source code (.tsx, .scss files)

The MCP server will search these directories for component documentation, source code, and examples to help with XMLUI development.

>[!TIP]
> When using agents with this MCP server, remind them frequently that you are working with XMLUI and want them to prioritize use of the tools provided by this server.

>[!TIP]
> The server launches with thConfigure your agents with rules like these, and reinforce with frequent reminders
>
> 1 don't write any code without my permission, always preview proposed changes, discuss, and only proceed with approval
>
> 2 don't add any xmlui styling, let the theme and layout engine do its job
>
> 3 proceed in small increments, write the absolute minimum amount of xmlui markup necessary and no script if possible
>
> 4 do not invent any xmlui syntax. only use constructs for which you can find examples in the docs and sample apps. cite your sources.
>
> 5 never touch the dom. we only work within xmlui abstractions inside the App realm, with help from vars and functions defined on the window variable in index.html
>
> 6 keep complex functions and expressions out of xmlui, then can live in index.html or (if scoping requires) in code-behind
>
> 7 use the xmlui mcp server to list and show component docs but also search xmlui source, docs, and examples
>
> 8 always do the simplest thing possible

## Install

To install, download the zip for your platform from [https://github.com/xmlui-org/xmlui-mcp/releases](https://github.com/xmlui-org/xmlui-mcp/releases), unzip, and cd into xmlui-mcp.

## Configure

If you run the server interactively, it says:

```
$ ./xmlui-mcp
Usage: ./xmlui-mcp <xmluiDir> [exampleRoot] [comma-separated-exampleDirs]
```

- **./xmlui-mcp** The server binary

- **xmluiDir** The directory where the binary lives, along with source and doc files searched by the binary.

- **exampleRoot** An optional directory to search for examples

- **comma-separated-exampleDirs** Subdirectories under exampleRoot

Here's how that maps into a configuration for Claude Desktop or Cursor.

```
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "/Users/jonudell/xmlui-invoice",
        "/Users/jonudell/xmlui-mastodon"
      ]
    },
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

With this setup I am giving the agents access to the XMLUI projects I'm working on (xmlui-invoice, xmlui-mastodon), so they can both read and write those projects.

I am also encouraging them to use the xmlui-mcp tools as we work on those projects. Here's what that looks like.

<img width="737" alt="image" src="https://github.com/user-attachments/assets/1f87519c-1338-4eca-a730-9f2e0c1a64a9" />

<img width="788" alt="image" src="https://github.com/user-attachments/assets/4793a475-46d1-418e-ad6a-0760af53ddca" />


### With VSCode

As of July 2025, VSCode work differently. Various sources claim you can use `~/.vscode.mcp` similar to above, we have not gotten that to work but this does by way of VSCode's `MCP: Add Server` command. If you need to adjust the settings it seems you need to uninstall and reinstall with a different command.

<img width="1311" height="751" alt="image" src="https://github.com/user-attachments/assets/7c7c368a-f930-41ed-a5eb-a2eaad419a25" />



## Test the server

You can test the server in HTTP mode to see what tools and prompts are available:

```bash
# Start server in HTTP mode
./xmlui-mcp --http /Users/jonudell/xmlui

# Test available tools
curl http://localhost:8080/tools

# Test available prompts
curl http://localhost:8080/prompts

# Get specific prompt content
curl http://localhost:8080/prompts/xmlui_rules
```

This will show you the same tools and prompts that MCP clients like Claude Desktop or Cursor can access.

When you ask an agent which XMLUI component to use, it has access to several tools:

- **xmlui_list_components** - Lists all available XMLUI components
- **xmlui_component_docs** - Gets documentation for a specific component
- **xmlui_search** - Searches XMLUI source code and documentation
- **xmlui_examples** - Finds usage examples in your projects
- **xmlui_read_file** - Reads specific files from the XMLUI codebase
- **xmlui-list-howto** - Lists available how-to guides
- **xmlui-search-howto** - Searches how-to documentation

The agent also has access to the **xmlui_rules** prompt, which contains essential guidelines for XMLUI development (similar to the TIP blocks above) to ensure it follows best practices and doesn't invent non-existent syntax.
