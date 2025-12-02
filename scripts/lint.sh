#!/usr/bin/env bash
set -Eeuo pipefail

LINTER="github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2"

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Running golangci-lint on all modules..."
while IFS= read -r mod_dir; do
  echo "  → $mod_dir"
  (cd "$mod_dir" && GOEXPERIMENT=jsonv2 go run ${LINTER} run ./... --timeout=5m)
done < <(get_module_dirs)

echo "✓ Lint complete"
