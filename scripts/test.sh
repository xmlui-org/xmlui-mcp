#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

RACE_FLAG=""
if [[ "$RACE" == "1" ]]; then
  RACE_FLAG="-race"
fi

echo "==> Running tests for all modules..."
while IFS= read -r mod_dir; do
  echo "  → $mod_dir"
  (cd "$mod_dir" && $GO test $RACE_FLAG -v ./...)
done < <(get_module_dirs)

echo "✓ All tests passed"
