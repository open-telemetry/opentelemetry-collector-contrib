#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

NAMESPACE="otel-demo"
SERVICE_NAME="otelcol-signing"
ENDPOINT="http://localhost:4318/v1/logs"

echo "Port-forwarding to $SERVICE_NAME..."
kubectl port-forward -n "$NAMESPACE" "service/$SERVICE_NAME" 4318:4318 &
PORT_FORWARD_PID=$!

# Ensure port-forward is stopped on exit
cleanup() {
  if kill -0 "$PORT_FORWARD_PID" 2>/dev/null; then
    kill "$PORT_FORWARD_PID"
    echo ""
    echo "Port-forward stopped."
  fi
}
trap cleanup EXIT INT TERM

sleep 3

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

echo ""
echo "Sending test log to $ENDPOINT..."
if curl -sf -X POST "$ENDPOINT" \
     -H "Content-Type: application/json" \
     -d "$LOG_BODY"; then
  echo ""
  echo "Log sent successfully!"
else
  echo ""
  echo "WARNING: Request may have succeeded (collector often returns empty response)"
fi

echo ""
echo "Checking collector logs for signatures..."
sleep 2
kubectl logs -n "$NAMESPACE" -l app="$SERVICE_NAME" --tail=50 | grep -i "audit.integrity" || true
