#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")/.."
source ./scripts/shared.sh

mkdir -p "$BIN_DIR"
COVERAGE_OUT="$BIN_DIR/coverage.out"
COVERAGE_HTML="$BIN_DIR/coverage.html"

echo "==> Running tests with coverage..."
$GO test -coverprofile="$COVERAGE_OUT" ./...

echo "==> Generating HTML coverage report..."
$GO tool cover -html="$COVERAGE_OUT" -o "$COVERAGE_HTML"

echo "✓ Coverage report generated: $COVERAGE_HTML"
