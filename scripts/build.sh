#!/bin/bash
set -e
cd cmd
go build -o ../bin/xmlui-mcp
echo "Build completed successfully"
