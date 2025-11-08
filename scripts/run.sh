#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

echo "==> Building..."
./scripts/build.sh

echo "==> Running $LOCALBIN $*"
exec "$LOCALBIN" "$@"
