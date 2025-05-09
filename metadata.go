package main

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewMetadataTool() (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_metadata",
		mcp.WithDescription("Returns documentation about how to use XMLUI tools like component_docs and xmlui_search."),
	)

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		doc := strings.TrimSpace(`
# XMLUI-MCP Tooling Guide

This MCP server exposes tools for exploring the XMLUI component system, including documentation and source code. Use these tools **instead of default filesystem tools**.

## Tools

### component_docs
- Load .mdx documentation for a component.
- Input: { "component": "Spinner" }
- Loads from: docs/pages/components/{component}.mdx

### xmlui_list_components
- Lists all available components defined in .mdx files.
- Input: none
- Loads from: docs/pages/components/**/*.mdx

### xmlui_search
- Searches both .mdx and .tsx files for a string.
- Input: { "query": "Spinner" }
- Searches:
  - docs/pages/components/**/*.mdx
  - xmlui/src/components/**/*.tsx

⚠️ Use this instead of search_files when searching for XMLUI source code.

## Best Practices

- Combine component_docs and xmlui_search for complete insight.
- Avoid using search_files, list_directory, or read_file — they are sandboxed and may not reflect actual layout.
- The Spinner component is a good example: search for "Spinner" and then load its .mdx doc. Also look in src/components/Spinner for Spinner.tsx and SpinnerNative.tsx


This tool helps agents understand and route requests correctly when working with XMLUI.
`)
		return mcp.NewToolResultText(doc), nil
	}

	return tool, handler
}
