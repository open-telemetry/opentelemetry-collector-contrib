#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GO_SCRIPT="$SCRIPT_DIR/verify-signed-log.go"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --log       Path to log JSON file (default: log.json in script dir)
  --cert      Path to PEM certificate file
  --hash      Hash algorithm: SHA256 | SHA512 (default: SHA256)
  --verbose   Enable verbose output
  --from-k8s  Fetch certificate from Kubernetes secret
  --namespace Kubernetes namespace (default: otel-demo)
  --secret    Kubernetes secret name (default: otelcol-test-certs)
  -h, --help  Show this help
EOF
  exit 0
}

LOG_FILE=""
CERT_FILE=""
HASH_ALG="SHA256"
VERBOSE=0
FROM_K8S=0
NAMESPACE="otel-demo"
SECRET_NAME="otelcol-test-certs"
TEMP_CERT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --log)       LOG_FILE="$2";    shift 2 ;;
    --cert)      CERT_FILE="$2";   shift 2 ;;
    --hash)      HASH_ALG="$2";    shift 2 ;;
    --verbose)   VERBOSE=1;        shift ;;
    --from-k8s)  FROM_K8S=1;       shift ;;
    --namespace) NAMESPACE="$2";   shift 2 ;;
    --secret)    SECRET_NAME="$2"; shift 2 ;;
    -h|--help)   usage ;;
    *) echo "Unknown option: $1" >&2; usage ;;
  esac
done

cleanup() {
  if [ -n "$TEMP_CERT" ] && [ -f "$TEMP_CERT" ]; then
    rm -f "$TEMP_CERT"
  fi
}
trap cleanup EXIT INT TERM

if [ ! -f "$GO_SCRIPT" ]; then
  echo "Error: verify-signed-log.go not found at $GO_SCRIPT" >&2
  exit 1
fi

if [ "$FROM_K8S" -eq 1 ]; then
  echo "Fetching certificate from Kubernetes secret..."
  CERT_DATA="$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.cert\.pem}' 2>/dev/null)" || {
    echo "Error: Failed to fetch certificate from secret $SECRET_NAME in namespace $NAMESPACE" >&2
    exit 1
  }
  TEMP_CERT="$(mktemp /tmp/verify-cert-XXXXXX.pem)"
  echo "$CERT_DATA" | base64 -d > "$TEMP_CERT"
  echo "Certificate saved to temporary file: $TEMP_CERT"
  CERT_FILE="$TEMP_CERT"
fi

if [ -z "$CERT_FILE" ]; then
  DEFAULT_CERT="$SCRIPT_DIR/cert.pem"
  if [ -f "$DEFAULT_CERT" ]; then
    CERT_FILE="$DEFAULT_CERT"
    echo "Using default certificate: $CERT_FILE"
  else
    echo "Error: Certificate file is required. Use --cert or --from-k8s" >&2
    echo "  Or place cert.pem in the same directory as this script" >&2
    exit 1
  fi
fi

if [ ! -f "$CERT_FILE" ]; then
  echo "Error: Certificate file not found: $CERT_FILE" >&2
  exit 1
fi

if [ -z "$LOG_FILE" ]; then
  DEFAULT_LOG="$SCRIPT_DIR/log.json"
  if [ -f "$DEFAULT_LOG" ]; then
    LOG_FILE="$DEFAULT_LOG"
    echo "Using default log file: $LOG_FILE"
  else
    echo "Error: Log file is required. Use --log" >&2
    exit 1
  fi
fi

EXE="$SCRIPT_DIR/verify-signed-log"
echo "Building verify-signed-log tool..."
go build -o "$EXE" "$GO_SCRIPT"

echo "Verifying signed log..."
echo ""

VERIFY_ARGS="-log $LOG_FILE -cert $CERT_FILE -hash $HASH_ALG"
if [ "$VERBOSE" -eq 1 ]; then
  VERIFY_ARGS="$VERIFY_ARGS -verbose"
fi

# shellcheck disable=SC2086
"$EXE" $VERIFY_ARGS
