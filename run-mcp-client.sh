#!/bin/bash

# Get the absolute path to the current directory
CWD="$(cd "$(dirname "$0")" && pwd)"

# First prepare the binaries
./prepare-binaries.sh

# Then run the client
./xmlui-mcp-client "$CWD/xmlui-mcp $CWD $CWD xmlui-invoice,xmlui-mastodon"
