#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

LOG_FILE="${1:-log.json}"

# jq is required for JSON parsing
if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq is required but not installed." >&2
  exit 1
fi

echo "=== Verification Diagnostic ==="
echo ""

if [ ! -f "$LOG_FILE" ]; then
  echo "Error: log file not found: $LOG_FILE" >&2
  exit 1
fi

BODY="$(jq -r '.body // ""' "$LOG_FILE")"
TIMESTAMP="$(jq -r '.timestamp // ""' "$LOG_FILE")"
SEV_NUMBER="$(jq -r '.severity_number // ""' "$LOG_FILE")"
SEV_TEXT="$(jq -r '.severity_text // ""' "$LOG_FILE")"
SIG_VAL="$(jq -r '.attributes["audit.integrity.value"] // ""' "$LOG_FILE")"
OTHER_COUNT="$(jq '[.attributes | to_entries[] | select(.key != "audit.integrity.value")] | length' "$LOG_FILE")"

echo "Log Record Data:"
echo "  Body: $BODY"
echo "  Timestamp: $TIMESTAMP"
echo "  Severity Number: $SEV_NUMBER"
echo "  Severity Text: $SEV_TEXT"
echo "  Attributes Count (excl. signature): $OTHER_COUNT"
echo ""

echo "Attributes (excluding signature):"
jq -r '.attributes | to_entries[] | select(.key != "audit.integrity.value") | "  \(.key): \(.value)"' "$LOG_FILE"
echo ""

echo "Signature from log (first 50 chars):"
echo "  ${SIG_VAL%"${SIG_VAL#??????????????????????????????????????????????????}"}..."
echo ""

echo "=== Verification Checklist ==="
echo ""

check() {
  LABEL="$1"
  RESULT="$2"
  if [ "$RESULT" = "true" ]; then
    echo "  [OK]   $LABEL"
  else
    echo "  [FAIL] $LABEL"
  fi
}

check "Body is present and is string"       "$([ -n "$BODY" ] && echo true || echo false)"
check "Timestamp is present and non-zero"   "$([ -n "$TIMESTAMP" ] && [ "$TIMESTAMP" != "0" ] && echo true || echo false)"
check "Severity number is present"          "$([ -n "$SEV_NUMBER" ] && echo true || echo false)"
check "Severity text is present"            "$([ -n "$SEV_TEXT" ] && echo true || echo false)"
check "Signature attribute exists"          "$([ -n "$SIG_VAL" ] && echo true || echo false)"
check "Other attributes present"            "$([ "$OTHER_COUNT" -gt 0 ] && echo true || echo false)"

echo ""
echo "=== Next Steps ==="
echo "1. Ensure all fields match exactly what was in the original log"
echo "2. Check if $LOG_FILE was created from the correct log record"
echo ""
echo "Run verification:"
echo "  ./verify-signed-log.sh"
