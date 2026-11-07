#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="otelcol-bench:local"
RESULTS_DIR="${SCRIPT_DIR}/results"

mkdir -p "$RESULTS_DIR"

echo "==> Building benchmark image..."
docker build -t "$IMAGE" "$SCRIPT_DIR"

echo "==> Running benchmarks (results → $RESULTS_DIR/bench.txt)..."
docker run --rm \
  -v "$RESULTS_DIR":/results \
  "$IMAGE"

echo ""
echo "==> Done. Results saved to $RESULTS_DIR/bench.txt"
