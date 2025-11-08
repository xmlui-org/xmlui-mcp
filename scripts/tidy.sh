#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Running go mod tidy for all modules..."
while IFS= read -r mod_dir; do
  echo "  → $mod_dir"
  (cd "$mod_dir" && $GO mod tidy)
done < <(get_module_dirs)

echo "✓ Tidy complete"
