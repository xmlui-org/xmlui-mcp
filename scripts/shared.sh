#!/usr/bin/env bash
set -Eeuo pipefail

# Shared defaults; override via env if needed.
BIN="${BIN:-xmlui-mcp}"
BIN_DIR="${BIN_DIR:-bin}"
CMD_DIR="${CMD_DIR:-cmd}"
PKG_DIR="${PKG_DIR:-xmluimcp}"
TEST_DIR="${TEST_DIR:-test}"

GO="${GO:-go}"
RACE="${RACE:-0}"

# Version metadata (override in CI if desired)
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 0000000)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# ldflags; append extra with LDFLAGS_APPEND if needed
LDFLAGS_BASE="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -X main.builtBy=make"
LDFLAGS="${LDFLAGS:-${LDFLAGS_BASE} ${LDFLAGS_APPEND:-}}"

LOCALBIN="${LOCALBIN:-${BIN_DIR}/${BIN}}"

# --- Dynamic discovery helpers -----------------------------------------------

# Discover all module roots (dirs containing go.mod), ignoring vendor/.git/bin/build
get_module_dirs() {
  find . -type f -name go.mod -print0 \
  | xargs -0 -n1 dirname \
  | sed 's#^\./##' \
  | grep -Ev '(^|/)(vendor|\.git|'"${BIN_DIR:-bin}"'|'"${BUILD_DIR:-build}"')(/|$)' \
  | sort -u
}

# Find directories with Go files (tests or not), then return ./<dir>/...
discover_test_directories() {
  local base_dirs=()
  while IFS= read -r -d '' dir; do
    # Skip vendor/.git/build/bin
    if [[ "$dir" == *"/vendor"* || "$dir" == *"/.git"* || "$dir" == *"/${BUILD_DIR:-build}"* || "$dir" == *"/${BIN_DIR:-bin}"* ]]; then
      continue
    fi
    if find "$dir" -maxdepth 1 -name "*.go" | grep -q .; then
      local rel_dir
      rel_dir=$(realpath --relative-to=. "$dir" 2>/dev/null || echo "$dir")
      base_dirs+=("./$rel_dir/...")
    fi
  done < <(find . -type d -print0)
  printf '%s\n' "${base_dirs[@]}" | sort -u
}

# Use provided args as dirs (normalized), else auto-discover
get_test_directories() {
  if [[ $# -gt 0 ]]; then
    for dir in "$@"; do
      [[ "$dir" != ./* ]] && dir="./$dir"
      [[ "$dir" != */... ]] && dir="$dir/..."
      echo "$dir"
    done
  else
    discover_test_directories
  fi
}
