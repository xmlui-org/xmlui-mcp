#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Formatting Go code..."
if command -v goimports &>/dev/null; then
  echo "  Using goimports"
  goimports -w .
else
  echo "  Using gofmt"
  gofmt -w .
fi

echo "✓ Formatting complete"
