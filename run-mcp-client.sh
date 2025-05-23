#!/bin/bash

# Get the absolute path to the current directory
CWD="$(cd "$(dirname "$0")" && pwd)"


# Use the first arg as examplesDir, default to ~
if [ -z "$1" ]; then
  echo "Using ~ as default example root, you can pass an alternate path to this script"
  EXAMPLES_DIR="$HOME"
else
  EXAMPLES_DIR="$1"
fi

echo

# First prepare the binaries
./prepare-binaries.sh

# Then run the client
./xmlui-mcp-client "$CWD/xmlui-mcp $CWD $EXAMPLES_DIR xmlui-invoice,xmlui-mastodon"
