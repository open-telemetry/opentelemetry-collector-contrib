#!/bin/bash
# check-mdatagen-generate.sh
# Verifies that every package with a metadata.yaml AND mdatagen-generated files
# has a //go:generate make mdatagen directive wired up via go generate.
# This ensures CI catches metadata.yaml changes that were not accompanied by
# running make mdatagen.

set -euo pipefail

REPO_ROOT="${1:-$(git rev-parse --show-toplevel)}"
FAILED=0

while IFS= read -r metadata_yaml; do
  dir=$(dirname "$metadata_yaml")

  # Only check dirs that contain mdatagen-generated files at the same level
  if ! find "$dir" -maxdepth 1 \( -name "generated_*.go" -o -name "generated_*.md" \) 2>/dev/null | grep -q .; then
    continue
  fi

  # Check for //go:generate make mdatagen in any .go file in the dir
  if ! grep -rl "go:generate make mdatagen" "$dir"/*.go 2>/dev/null | grep -q .; then
    echo "ERROR: $dir has metadata.yaml and generated files but is missing '//go:generate make mdatagen' in a .go file (e.g. doc.go)."
    FAILED=1
  fi
done < <(find "$REPO_ROOT" -name "metadata.yaml" -not -path "*/vendor/*")

if [ "$FAILED" -ne 0 ]; then
  echo ""
  echo "To fix: add a doc.go file with '//go:generate make mdatagen' to each listed package."
  echo "This ensures 'make generate' (run by CI) regenerates mdatagen outputs when metadata.yaml changes."
  exit 1
fi

echo "All packages with metadata.yaml + generated files have //go:generate make mdatagen wired up."
