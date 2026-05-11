#!/usr/bin/env bash
#
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
#

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HARNESS_DIR="${ROOT_DIR}/test/saw7500"

ARTIFACT_ROOT="${ROOT_DIR}/artifacts/saw-7500/sweep-$(date -u +%Y%m%dT%H%M%SZ)"
TARGETS_CSV="131072,262144,524288,1048576"
PHASE="green"
RUN_ARGS=()

usage() {
  cat <<'EOF'
Run the SAW-7500 request-size sweep.

Usage:
  test/saw7500/sweep.sh [options] [-- run.sh options]

Options:
  --artifacts DIR       Sweep artifact root.
  --targets CSV         Compressed targets. Default: 131072,262144,524288,1048576.
  --phase PHASE         run.sh phase. Default: green.
  -h, --help            Show this help.

Examples:
  test/saw7500/sweep.sh -- --payload-profile repeated --payload-size-bytes 1048576
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --artifacts) ARTIFACT_ROOT="$2"; shift 2 ;;
    --targets) TARGETS_CSV="$2"; shift 2 ;;
    --phase) PHASE="$2"; shift 2 ;;
    --) shift; RUN_ARGS+=("$@"); break ;;
    -h|--help) usage; exit 0 ;;
    *) RUN_ARGS+=("$1"); shift ;;
  esac
done

IFS=',' read -r -a TARGETS <<< "${TARGETS_CSV}"
mkdir -p "${ARTIFACT_ROOT}"
overall_status=0

for target in "${TARGETS[@]}"; do
  target="$(echo "${target}" | tr -d '[:space:]')"
  [[ -n "${target}" ]] || continue
  echo "=== target_compressed_bytes=${target} ==="
  if ! "${HARNESS_DIR}/run.sh" \
    --phase "${PHASE}" \
    --target-compressed-bytes "${target}" \
    --artifacts "${ARTIFACT_ROOT}/target-${target}" \
    "${RUN_ARGS[@]}"; then
    overall_status=1
    echo "target_compressed_bytes=${target} failed; continuing sweep" >&2
  fi
done

python3 "${HARNESS_DIR}/sweep.py" \
  --artifacts "${ARTIFACT_ROOT}" \
  --output "${ARTIFACT_ROOT}/sweep.md" \
  --json-output "${ARTIFACT_ROOT}/sweep.json"

echo "sweep artifacts: ${ARTIFACT_ROOT}"
echo "sweep report: ${ARTIFACT_ROOT}/sweep.md"
exit "${overall_status}"
