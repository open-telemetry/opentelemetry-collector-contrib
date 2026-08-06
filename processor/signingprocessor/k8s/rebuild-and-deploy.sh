#!/bin/sh
# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
IMAGE_NAME="otelcol-signing:latest"
DOCKERFILE_PATH="processor/signingprocessor/k8s/Dockerfile.signing"
DEPLOYMENT_FILE="$SCRIPT_DIR/otelcol-signing-k8s-secret.yaml"

echo "========================================"
echo "Rebuild and Deploy Collector"
echo "========================================"
echo ""

echo "Step 1: Building Docker image..."
echo "  Compiling the collector with latest code changes..."
echo "  Building from: $REPO_ROOT"
cd "$REPO_ROOT"
docker build -f "$DOCKERFILE_PATH" -t "$IMAGE_NAME" .
echo "Image built successfully!"

echo ""
echo "Step 2: Checking cluster type..."
CLUSTER_CTX="$(kubectl config current-context 2>&1 || true)"
case "$CLUSTER_CTX" in
  *kind*)
    echo "Detected kind cluster, loading image..."
    if kind load docker-image "$IMAGE_NAME"; then
      echo "Image loaded into kind cluster!"
    else
      echo "WARNING: Failed to load image into kind. Continuing anyway..."
    fi
    ;;
  *)
    echo "Not a kind cluster. If using a remote cluster, push the image to a registry."
    ;;
esac

echo ""
echo "Step 3: Deleting existing pods to force new image pull..."
kubectl delete pod -n otel-demo -l app=otelcol-signing --ignore-not-found=true
echo "Old pods deleted"

echo ""
echo "Step 4: Applying deployment..."
kubectl apply -f "$DEPLOYMENT_FILE"
echo "Deployment applied successfully!"

echo ""
echo "Step 5: Waiting for pod to be ready..."
if kubectl wait --for=condition=ready pod -n otel-demo -l app=otelcol-signing --timeout=60s; then
  echo "Pod is ready!"
else
  echo "WARNING: Pod may not be ready yet. Check with: kubectl get pods -n otel-demo"
fi

echo ""
echo "Step 6: Checking pod status..."
kubectl get pods -n otel-demo -l app=otelcol-signing

echo ""
echo "Step 7: Checking logs for initialization..."
sleep 2
LOGS="$(kubectl logs -n otel-demo -l app=otelcol-signing --tail=10 2>&1 || true)"
case "$LOGS" in
  *"Using Kubernetes secret"*|*"Everything is ready"*)
    echo "SUCCESS: Collector is running with K8s secret!"
    ;;
  *)
    echo "Recent logs:"
    echo "$LOGS" | sed 's/^/  /'
    ;;
esac

echo ""
echo "========================================"
echo "Deployment Complete!"
echo "========================================"
echo ""
echo "To view logs:"
echo "  kubectl logs -n otel-demo -l app=otelcol-signing -f"
echo ""
echo "To test:"
echo "  ./test-send-log.sh"
