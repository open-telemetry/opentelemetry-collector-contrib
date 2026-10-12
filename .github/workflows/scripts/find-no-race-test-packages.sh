#!/usr/bin/env bash
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#
# find-no-race-test-packages.sh
#
# Finds Go packages in the given directory (default: current dir) that contain
# test files guarded by //go:build !race.
#
# Usage:
#   find-no-race-test-packages.sh [search_dir]
#
# Output:
#   A space-separated list of unique package import paths (./pkg/...) suitable
#   for passing directly to gotestsum --packages. Outputs nothing and exits 0
#   if none are found.

set -euo pipefail

SEARCH_DIR="${1:-.}"

# Find all *_test.go files that carry a //go:build !race constraint, extract
# their unique parent directories, then convert to ./relative paths.
#
# Notes:
#  - `|| true` prevents grep's exit code 1 (no matches) from aborting the
#    script under `set -e` / `set -o pipefail`.
#  - `while IFS= read -r f` safely handles file paths that contain spaces.
#  - `sort -u` deduplicates packages when multiple !race test files share one.
PKGS=$(find "$SEARCH_DIR" -name '*_test.go' -exec grep -lE '^//go:build.*!race' {} + 2>/dev/null \
    | while IFS= read -r f; do dirname "$f"; done \
    | sort -u || true)

if [ -z "${PKGS:-}" ]; then
    exit 0
fi

echo "$PKGS"
