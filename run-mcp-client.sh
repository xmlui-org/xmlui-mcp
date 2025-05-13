#!/bin/bash

# Make the binaries executable
chmod +x xmlui-mcp-client
chmod +x xmlui-mcp

# On macOS, remove quarantine attribute if present
if [[ "$(uname)" == "Darwin" ]]; then
  xattr -d com.apple.quarantine xmlui-mcp-client 2>/dev/null || true
  xattr -d com.apple.quarantine xmlui-mcp 2>/dev/null || true
fi

# for example...
./xmlui-mcp-client "/Users/jonudell/xmlui-getting-started/mcp/xmlui-mcp /Users/jonudell/xmlui-getting-started/mcp /Users/jonudell xmlui-hn,xmlui-mastodon,xmlui-cms"
