#!/bin/bash

set -e

echo "Deploying OpenTelemetry Collector with Isolation Forest to Kubernetes..."

# Build and push container
echo "Building container..."
./scripts/build_container.sh

# Replace with your registry
REGISTRY="your-registry.com"
IMAGE_NAME="otelcontribcol-isolation-forest"
IMAGE_TAG="v0.131.0-$(git rev-parse --short HEAD)"

# Tag and push to registry
echo "Pushing to registry..."
docker tag ${IMAGE_NAME}:${IMAGE_TAG} ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}
docker tag ${IMAGE_NAME}:${IMAGE_TAG} ${REGISTRY}/${IMAGE_NAME}:latest
docker push ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}
docker push ${REGISTRY}/${IMAGE_NAME}:latest

# Update deployment image
sed -i.bak "s|image: ${REGISTRY}/otelcontribcol-isolation-forest:latest|image: ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}|g" k8s/deployment.yaml

# Apply Kubernetes manifests
echo "Applying Kubernetes manifests..."
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Optional: Apply ServiceMonitor if you have Prometheus Operator
if kubectl get crd servicemonitors.monitoring.coreos.com >/dev/null 2>&1; then
    kubectl apply -f k8s/servicemonitor.yaml
    echo "ServiceMonitor applied"
fi

echo "Deployment complete!"
echo "Checking deployment status..."
kubectl -n observability get pods -l app=otelcol-isolation-forest
kubectl -n observability get svc otelcol-isolation-forest

echo "To check logs:"
echo "kubectl -n observability logs -l app=otelcol-isolation-forest -f"

echo "To port-forward for testing:"
echo "kubectl -n observability port-forward svc/otelcol-isolation-forest 4317:4317 4318:4318 8888:8888"
