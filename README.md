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
> Configure your agents with rules like these, and reinforce with frequent reminders
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

You can run the bundled client to see what it's like for an agent to use the server.

```
./run-mcp-client.sh
Using ~ as default example root, you can pass an alternate path to this script

Connected to server with tools: [xmlui_component_docs xmlui_examples xmlui_list_components xmlui_read_file xmlui_search]

Type 'help' for commands, 'quit' to exit.
>
> help

Available commands:
  list              - List all XMLUI components
  docs <name>       - Docs for a component
  search <term>     - Search XMLUI code/docs
  read <path>       - Read a file
  examples <query>  - Search usage examples
  howtolist         - List all 'How To' entry titles
  howtosearch <q>   - Search 'How To' entries by keyword
  help              - Show this help
  quit              - Quit
```

If you ask an agent which XMLUI component to use, it might start with the xmlui_list_components tool.

```
> list

## APICall

- APICall → call xmlui_docs with component: "APICall"

## App

- App → call xmlui_docs with component: "App"

> docs APICall

> docs APICall

# APICall [#component-apicall]

`APICall` is used to mutate (create, update or delete) some data on the backend. It is similar in nature to the `DataSource` component which retrieves data from the backend.
```

It might follow up by using the xmlui_component_docs tool to get the doc for a given component.

```
> docs slider

# Slider

The `Slider` component allows you to select a numeric value between a range specified by minimum and maximum values.

## Properties

### `autoFocus (default: false)`

If this property is set to `true`, the component gets the focus automatically when displayed.

### `enabled (default: true)`

This boolean property value indicates whether the component responds to user events (`true`) or not (`false`).
```

It might decide to use the xmlui_search tool to search docs and code.

```
> search slider

docs/pages/components/Slider.mdx:3: # Slider [#component-slider]
docs/pages/components/Slider.mdx:7: The `Slider` component allows you to select a numeric value between a range specified by minimum and maximum values.
src/components/ComponentProvider.tsx:123: import { sliderComponentRenderer } from "./Slider/Slider";
src/components/FormItem/FormItemNative.tsx:51: import { Slider } from "../Slider/SliderNative";
src/components/Slider/Slider.mdx:1: # Slider
src/components/Slider/Slider.tsx:1: import styles from "./Slider.module.scss";
src/components/Slider/SliderNative.tsx:4: import { Root, Range, Track, Thumb } from "@radix-ui/react-slider";}
```

It might use the xmlui_examples tool to find uses in one or more app directories.

```
> examples slider

## File: xmlui-invoice/components/DailyRevenue.xmlui

### Matching Lines:

   28: <Slider
   29: id="slider"
   36: console.log('slider values:', slider.value[0], slider.value[1]);
   37: // Update the start and end dates based on slider values
   38: startDate = window.sliderValueToDate(slider.value[0]);
   39: endDate = window.sliderValueToDate(slider.value[1]);

### Complete File:

```xml
<Component
  name="DailyRevenue"
...
</Component>
```

It might list and search How To articles.

```
>  howtosearch delay datasource
```

That will find and cite **Source:** https://docs.xmlui.org/howto#hide-an-element-until-its-datasource-is-ready
