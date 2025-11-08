#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Cleaning build artifacts..."
if [[ -d "$BIN_DIR" ]]; then
  rm -rf "$BIN_DIR"
  echo "  Removed $BIN_DIR/"
fi

echo "✓ Clean complete"
