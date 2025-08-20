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
   - `$HOME/xmlui/docs/public/pages/howto.md` - HowTo docs with working playgrounds
   - `$HOME/xmlui/xmlui/src/components/` - Source code (.tsx, .scss files)

The MCP server will search these directories for component documentation, source code, and examples to help with XMLUI development.

> [!TIP]
> The rules injected into the agent's session tell it to prioritize searching and citing How To articles which lead to non-hallucinated known-working examples. But it helps to say that explicitly, and in general to often say "remember the rules" because LLMs are forgetful and need constant reminding.

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

Agents like Claude, Cursor, and Copilot typically use the server in stdio mode.

You can run the server in HTTP mode to see what tools and prompts are available.

```
~/xmlui-mcp$ ./xmlui-mcp --http  ~/xmlui ~  xmlui-invoice,xmlui-mastodon
PROMPT: xmlui_rules
 Essential rules and guidelines for XMLUI development

xmlui_list_components
 Lists all available XMLUI components based on .md files in docs/content/components.

xmlui_component_docs
 Returns the Markdown documentation for a given XMLUI component from docs/content/components.
 Input schema:
   - component (required): Component name, e.g. 'Button', 'Avatar', or 'Stack/VStack'

xmlui_search
 Searches XMLUI source and documentation files.
 Input schema:
   - query (required): Search term, e.g. 'Slider' or 'boxShadow'

xmlui_read_file
 Reads a .mdx, .tsx, .scss, or .md file from the XMLUI source or docs tree.
 Input schema:
   - path (required): Relative path under docs/content/components, xmlui/src/components, or docs/public/pages, e.g. 'xmlui/src/components/Spinner/Spinner.tsx'

Example roots configured: [/Users/jonudell/xmlui-invoice /Users/jonudell/xmlui-mastodon]
xmlui_examples
 Searches local sample apps for usage examples of XMLUI components. Provide a query string to search for.
 Input schema:
   - query (required): Search term, e.g. 'Spinner', 'AppState', or 'delay="1000"'

xmlui_list_howto
 List all 'How To' entry titles from docs/public/pages/howto.md.

xmlui_search_howto
 Search for 'How To' entries in docs/public/pages/howto.md by keyword or phrase. Returns full markdown sections.
 Input schema:
   - query (required): Keyword or phrase to search for.

xmlui_inject_prompt
 Inject a prompt into the current session context for guidance
 Input schema:
   - prompt_name (required): Name of the prompt to inject (e.g., 'xmlui_rules')
   - session_id : Session ID (optional, defaults to 'default')

xmlui_list_prompts
 Lists all available prompts that can be injected into session context

xmlui_get_prompt
 Retrieves the content of a specific prompt for review
 Input schema:
   - prompt_name (required): Name of the prompt to retrieve (e.g., 'xmlui_rules')

Starting HTTP server on port 8080
SSE endpoint: http://localhost:8080/sse
Message endpoint: http://localhost:8080/message
Tools endpoint: http://localhost:8080/tools
Prompts list endpoint: http://localhost:8080/prompts
Specific prompt endpoint: http://localhost:8080/prompts/{name}
Inject prompt endpoint: http://localhost:8080/session/context
Analytics summary endpoint: http://localhost:8080/analytics/summary


# Test available tools
curl http://localhost:8080/tools

# Test available prompts
curl http://localhost:8080/prompts

# Get specific prompt content
curl http://localhost:8080/prompts/xmlui_rules

# Inject prompt into session ("remember the rules" workflow)
curl -X POST http://localhost:8080/session/context \
  -H "Content-Type: application/json" \
  -d '{"prompt_name": "xmlui_rules"}'

# View analytics summary
curl http://localhost:8080/analytics/summary
```

## Analytics

The server now includes comprehensive analytics to track agent usage patterns and optimize the tools.

- Analytics are automatically collected when agents use the server

- Data is saved to `xmlui-mcp-analytics.json`

- View analytics with: `./analytics-helper.sh summary`

- Access via HTTP endpoints when running with `--http`

# Rules

The server contains these rules:

```
1 don't write any code without my permission, always preview proposed changes, discuss, and only proceed with approval.

2 don't add any xmlui styling, let the theme and layout engine do its job

3 proceed in small increments, write the absolute minimum amount of xmlui markup necessary and no script if possible

4 do not invent any xmlui syntax. only use constructs for which you can find examples in the docs and sample apps. cite your sources.

5 never touch the dom. we only work within xmlui abstractions inside the <App> realm, with help from vars and functions defined on the window variable in index.html

6 keep complex functions and expressions out of xmlui, then can live in index.html or (if scoping requires) in code-behind

7 use the xmlui mcp server to list and show component docs but also search xmlui source, docs, and examples

8 always do the simplest thing possible

9 use a neutral tone. do not say "Perfect!" etc. in fact never use exclamation marks at all

10 when creating examples for live playgrounds, observe the conventions for ---app and ---comp

11 VStack is the default, don't use it unless necessary

12 always search XMLUI-related resources first and prioritize them over other sources
```

Agents should see these rules when starting the xmlui-mcp server but they are forgetful and when they do forget you can try saying "remember the rules" to reinject them into your session's context.

