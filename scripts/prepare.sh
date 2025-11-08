#!/bin/bash

# Make the binary executable
chmod +x bin/xmlui-mcp

# On macOS, remove quarantine attribute if present
if [[ "$(uname)" == "Darwin" ]]; then
  xattr -d com.apple.quarantine bin/xmlui-mcp 2>/dev/null || true
  echo "✓ Quarantine bits removed from MCP binary"
fi
