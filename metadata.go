package main

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewMetadataTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_metadata",
		mcp.WithDescription("Returns documentation about how to use XMLUI tools like xmlui_component_docs and xmlui_search."),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		doc := strings.TrimSpace(`
# XMLUI-MCP Tooling Guide

This MCP server exposes tools for exploring the XMLUI component system, including documentation and source code. Use these tools **instead of default filesystem tools**.

## Tools

### xmlui_component_docs
- Load .mdx documentation for a component.
- Input: { "component": "Spinner" }
- Loads from: docs/pages/components/{component}.mdx

### xmlui_list_components
- List all available components defined in .mdx files.
- Input: none
- Loads from: docs/pages/components/**/*.mdx

### xmlui_search
- Searches both .mdx and .tsx files for a string.
- Input: { "query": "Spinner" }
- Searches:
  - docs/pages/components/**/*.mdx
  - xmlui/src/components/**/*.tsx

### xmlui_examples
- Search configured local sample repos  string.
- Input: { "query": "Spinner" }

### xmlui_read_file
- Reads a .mdx or .tsx or .scss file from the XMLUI source or docs tree.
- Input: { "path": "~/xmlui-invoice" }

⚠️ Use this instead of read_files when reading XMLUI source and docs

## Best Practices

- Combine component_docs, xmlui_search, and xmlui_examples for complete insight.
- Avoid using search_files, list_directory, or read_file — they are sandboxed and may not reflect actual layout.
- The Slider component is a good example: search for "Spinner", load its .mdx doc. Also look in src/components/Spinner for Spinner.tsx and SpinnerNative.tsx, and find examples in local sample repos.


This tool helps agents understand and route requests correctly when working with XMLUI.
`)
		return mcp.NewToolResultText(doc), nil
	}

	return tool, handler
}
