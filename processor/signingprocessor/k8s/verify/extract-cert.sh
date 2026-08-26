#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --source      k8s | openbao  (default: k8s)
  --namespace   Kubernetes namespace (default: otel-demo)
  --secret      Kubernetes secret name (default: otelcol-test-certs)
  --bao-addr    OpenBao address (default: http://openbao.openbao.svc.cluster.local:8200)
  --bao-path    OpenBao secret path (default: certs/data/test1)
  --bao-token   OpenBao token (required for openbao source)
  --bao-ns      OpenBao namespace header (optional)
  --output-dir  Output directory (default: current directory)
  --cert-only   Extract certificate only (skip key and CA)
  --all         Extract cert, key, and CA
  -h, --help    Show this help
EOF
  exit 0
}

SOURCE="k8s"
NAMESPACE="otel-demo"
SECRET_NAME="otelcol-test-certs"
BAO_ADDR="http://openbao.openbao.svc.cluster.local:8200"
BAO_PATH="certs/data/test1"
BAO_TOKEN=""
BAO_NS=""
OUTPUT_DIR="."
CERT_ONLY=0

while [ $# -gt 0 ]; do
  case "$1" in
    --source)    SOURCE="$2";    shift 2 ;;
    --namespace) NAMESPACE="$2"; shift 2 ;;
    --secret)    SECRET_NAME="$2"; shift 2 ;;
    --bao-addr)  BAO_ADDR="$2";  shift 2 ;;
    --bao-path)  BAO_PATH="$2";  shift 2 ;;
    --bao-token) BAO_TOKEN="$2"; shift 2 ;;
    --bao-ns)    BAO_NS="$2";    shift 2 ;;
    --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
    --cert-only) CERT_ONLY=1;    shift ;;
    --all)       CERT_ONLY=0;    shift ;;
    -h|--help)   usage ;;
    *) echo "Unknown option: $1" >&2; usage ;;
  esac
done

echo "========================================"
echo "Extract Certificate"
echo "========================================"
echo ""

mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"

extract_from_k8s() {
  echo "Extracting certificate from Kubernetes secret..."
  echo "  Secret: $SECRET_NAME"
  echo "  Namespace: $NAMESPACE"
  echo ""

  SECRET_JSON="$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o json 2>&1)" || {
    echo "Error: Failed to get secret '$SECRET_NAME' from namespace '$NAMESPACE'" >&2
    return 1
  }

  if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required but not installed." >&2
    return 1
  fi

  echo "Available keys in secret: $(echo "$SECRET_JSON" | jq -r '.data | keys | join(", ")')"
  echo ""

  CERT_DATA="$(echo "$SECRET_JSON" | jq -r '.data["cert.pem"] // ""')"
  if [ -n "$CERT_DATA" ]; then
    echo "$CERT_DATA" | base64 -d > "$OUTPUT_DIR/cert.pem"
    echo "  [OK] Certificate saved to: $OUTPUT_DIR/cert.pem"
  else
    echo "  [WARN] cert.pem not found in secret"
    return 1
  fi

  if [ "$CERT_ONLY" -eq 0 ]; then
    KEY_DATA="$(echo "$SECRET_JSON" | jq -r '.data["key.pem"] // ""')"
    if [ -n "$KEY_DATA" ]; then
      echo "$KEY_DATA" | base64 -d > "$OUTPUT_DIR/key.pem"
      echo "  [OK] Private key saved to: $OUTPUT_DIR/key.pem"
    fi

    CA_DATA="$(echo "$SECRET_JSON" | jq -r '.data["ca.pem"] // ""')"
    if [ -n "$CA_DATA" ]; then
      echo "$CA_DATA" | base64 -d > "$OUTPUT_DIR/ca.pem"
      echo "  [OK] CA certificate saved to: $OUTPUT_DIR/ca.pem"
    fi
  fi
}

extract_from_openbao() {
  echo "Extracting certificate from OpenBao..."
  echo "  Address: $BAO_ADDR"
  echo "  Path: $BAO_PATH"
  echo ""

  if [ -z "$BAO_TOKEN" ]; then
    echo "Error: OpenBao token is required. Use --bao-token." >&2
    return 1
  fi

  CURL_ARGS="-sf -H X-Vault-Token: $BAO_TOKEN"
  if [ -n "$BAO_NS" ]; then
    CURL_ARGS="$CURL_ARGS -H X-Vault-Namespace: $BAO_NS"
  fi

  RESPONSE="$(curl -sf \
    -H "X-Vault-Token: $BAO_TOKEN" \
    ${BAO_NS:+-H "X-Vault-Namespace: $BAO_NS"} \
    "$BAO_ADDR/v1/$BAO_PATH")" || {
    echo "Error fetching from OpenBao" >&2
    return 1
  }

  if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is required but not installed." >&2
    return 1
  fi

  # Try nested data.data first (KV v2), then data (KV v1)
  CERT_PEM="$(echo "$RESPONSE" | jq -r '.data.data.certificate // .data.certificate // .data.data.cert // .data.cert // ""')"
  if [ -z "$CERT_PEM" ]; then
    echo "Error: Certificate not found in OpenBao response" >&2
    return 1
  fi
  printf '%s\n' "$CERT_PEM" > "$OUTPUT_DIR/cert.pem"
  echo "  [OK] Certificate saved to: $OUTPUT_DIR/cert.pem"

  if [ "$CERT_ONLY" -eq 0 ]; then
    KEY_PEM="$(echo "$RESPONSE" | jq -r '.data.data.private_key // .data.private_key // .data.data.key // .data.key // ""')"
    if [ -n "$KEY_PEM" ]; then
      printf '%s\n' "$KEY_PEM" > "$OUTPUT_DIR/key.pem"
      echo "  [OK] Private key saved to: $OUTPUT_DIR/key.pem"
    fi

    CA_PEM="$(echo "$RESPONSE" | jq -r '(.data.data.ca_chain // .data.ca_chain // .data.data.ca // .data.ca // []) | if type == "array" then join("\n") else . end')"
    if [ -n "$CA_PEM" ]; then
      printf '%s\n' "$CA_PEM" > "$OUTPUT_DIR/ca.pem"
      echo "  [OK] CA certificate saved to: $OUTPUT_DIR/ca.pem"
    fi
  fi
}

case "$SOURCE" in
  k8s)     extract_from_k8s ;;
  openbao) extract_from_openbao ;;
  *)
    echo "Error: source must be 'k8s' or 'openbao'" >&2
    exit 1
    ;;
esac

echo ""
echo "========================================"
echo "Extraction Complete"
echo "========================================"
echo ""
echo "Certificate saved to: $OUTPUT_DIR"
echo ""
echo "You can now verify logs with:"
echo "  ./verify-signed-log.sh"
