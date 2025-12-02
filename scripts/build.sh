#!/bin/bash
set -e
cd cmd
GOEXPERIMENT=jsonv2 go build -o ../bin/xmlui-mcp
echo "Build completed successfully"
