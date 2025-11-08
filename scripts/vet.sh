#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Running go vet on all modules..."
while IFS= read -r mod_dir; do
  echo "  → $mod_dir"
  (cd "$mod_dir" && $GO vet ./...)
done < <(get_module_dirs)

echo "✓ Vet complete"
