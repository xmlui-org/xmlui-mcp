#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."

# Pass all arguments to analytics-helper.sh
exec ./scripts/analytics-helper.sh "$@"
