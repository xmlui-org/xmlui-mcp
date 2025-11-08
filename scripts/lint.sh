#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

if command -v golangci-lint &>/dev/null; then
  echo "==> Running golangci-lint..."
  golangci-lint run ./...
  echo "✓ Lint complete"
else
  echo "==> golangci-lint not found, falling back to go vet..."
  ./scripts/vet.sh
fi
