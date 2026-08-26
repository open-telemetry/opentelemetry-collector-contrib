#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --namespace   Kubernetes namespace (default: otel-demo)
  --service     Collector service name (default: otelcol-signing)
  --output      Output JSON file (default: log-from-collector.json)
  -h, --help    Show this help
EOF
  exit 0
}

NAMESPACE="otel-demo"
SERVICE_NAME="otelcol-signing"
OUTPUT_FILE="log-from-collector.json"

while [ $# -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="$2";    shift 2 ;;
    --service)   SERVICE_NAME="$2"; shift 2 ;;
    --output)    OUTPUT_FILE="$2";  shift 2 ;;
    -h|--help)   usage ;;
    *) echo "Unknown option: $1" >&2; usage ;;
  esac
done

if ! command -v jq >/dev/null 2>&1; then
  echo "Error: jq is required but not installed." >&2
  exit 1
fi

echo "========================================"
echo "Test and Verify Signed Log"
echo "========================================"
echo ""

ENDPOINT="http://localhost:4318/v1/logs"

echo "Step 1: Port-forwarding to $SERVICE_NAME..."
kubectl port-forward -n "$NAMESPACE" "service/$SERVICE_NAME" 4318:4318 &
PORT_FORWARD_PID=$!

cleanup() {
  if kill -0 "$PORT_FORWARD_PID" 2>/dev/null; then
    kill "$PORT_FORWARD_PID"
    echo ""
    echo "Port-forward stopped."
  fi
}
trap cleanup EXIT INT TERM

sleep 5

echo "Step 2: Sending test log to $ENDPOINT..."

TIMESTAMP_NS="$(date +%s)000000000"
LOG_BODY=$(cat <<EOF
{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          { "key": "service.name", "value": { "stringValue": "test-service" } }
        ]
      },
      "scopeLogs": [
        {
          "scope": {},
          "logRecords": [
            {
              "timeUnixNano": "$TIMESTAMP_NS",
              "severityNumber": 9,
              "severityText": "INFO",
              "body": { "stringValue": "Test log message from shell" },
              "attributes": [
                { "key": "test.attribute", "value": { "stringValue": "test-value" } }
              ]
            }
          ]
        }
      ]
    }
  ]
}
EOF
)

if curl -sf -X POST "$ENDPOINT" \
     -H "Content-Type: application/json" \
     -d "$LOG_BODY" >/dev/null 2>&1; then
  echo "  [OK] Log sent successfully!"
else
  echo "  [WARN] Request may have succeeded (collector often returns empty response)"
fi

echo ""
echo "Step 3: Waiting for collector to process log..."
sleep 3

echo "Step 4: Extracting signed log from collector output..."
LOGS="$(kubectl logs -n "$NAMESPACE" -l app="$SERVICE_NAME" -c otelcol --tail=200 2>&1 || true)"

if [ -z "$LOGS" ]; then
  echo "  [ERROR] No logs found from collector" >&2
  exit 1
fi

# Parse the debug exporter text format line by line using awk
LOG_JSON="$(echo "$LOGS" | awk '
/LogRecord #/ { in_record=1; body=""; timestamp=""; sev_num=""; sev_text=""; delete attrs; in_attrs=0 }
in_record && /Body:.*Str\(/ {
  match($0, /Str\(([^)]+)\)/, m)
  body=m[1]
}
in_record && /Timestamp:/ {
  match($0, /([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2})/, m)
  timestamp=m[1]
}
in_record && /SeverityText:/ {
  match($0, /SeverityText:[[:space:]]*([A-Z]+)/, m)
  sev_text=m[1]
}
in_record && /SeverityNumber:.*\(([0-9]+)\)/ {
  match($0, /\(([0-9]+)\)/, m)
  sev_num=m[1]
}
in_record && /^Attributes:/ { in_attrs=1 }
in_record && in_attrs && /->.*:.*Str\(/ {
  match($0, /-> ([^:]+): Str\(([^)]+)\)/, m)
  attrs[m[1]]=m[2]
}
END {
  if (body == "Test log message from shell" && ("audit.integrity.value" in attrs)) {
    printf "{\n"
    printf "  \"body\": \"%s\",\n", body
    printf "  \"timestamp\": \"%s\",\n", timestamp
    printf "  \"severity_number\": %s,\n", (sev_num ? sev_num : "null")
    printf "  \"severity_text\": \"%s\",\n", sev_text
    printf "  \"attributes\": {\n"
    sep=""
    for (k in attrs) {
      printf "%s    \"%s\": \"%s\"", sep, k, attrs[k]
      sep=",\n"
    }
    printf "\n  }\n}\n"
  }
}
')"

if [ -z "$LOG_JSON" ]; then
  echo "  [ERROR] Could not extract signed log from collector output" >&2
  echo "  Searching for audit.integrity in logs..."
  echo "$LOGS" | grep -i "audit.integrity" | head -10 || echo "  Not found."
  echo ""
  echo "  Searching for test message..."
  echo "$LOGS" | grep -i "Test log message" | head -5 || echo "  Not found."
  echo ""
  echo "  Full collector logs (last 50 lines):"
  echo "$LOGS" | tail -50
  exit 1
fi

echo "  [OK] Found signed log record"

OUTPUT_PATH="$(pwd)/$OUTPUT_FILE"
printf '%s\n' "$LOG_JSON" > "$OUTPUT_PATH"
echo "  [OK] Log saved to: $OUTPUT_PATH"

echo ""
echo "Step 5: Verifying signed log..."
echo ""

"$SCRIPT_DIR/verify-signed-log.sh" --log "$OUTPUT_FILE" --from-k8s --namespace "$NAMESPACE"
