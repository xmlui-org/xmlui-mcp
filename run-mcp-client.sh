#!/bin/bash

# Make both binaries executable
chmod +x xmlui-mcp xmlui-mcp-client

# On macOS, remove quarantine attribute
if [[ "$(uname)" == "Darwin" ]]; then
  xattr -d com.apple.quarantine xmlui-mcp 2>/dev/null || true
  xattr -d com.apple.quarantine xmlui-mcp-client 2>/dev/null || true
fi

# for example...
./xmlui-mcp-client "/Users/jonudell/xmlui-mcp/xmlui-mcp /Users/jonudell/xmlui /Users/jonudell xmlui-hn,xmlui-mastodon,xmlui-invoice"



