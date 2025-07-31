#!/bin/bash

set -e

echo "Testing OpenTelemetry Collector deployment..."

# Wait for pods to be ready
echo "Waiting for pods to be ready..."
kubectl -n observability wait --for=condition=ready pod -l app=otelcol-isolation-forest --timeout=300s

# Port forward in background
echo "Setting up port forwards..."
kubectl -n observability port-forward svc/otelcol-isolation-forest 4317:4317 &
PF_GRPC_PID=$!
kubectl -n observability port-forward svc/otelcol-isolation-forest 4318:4318 &
PF_HTTP_PID=$!
kubectl -n observability port-forward svc/otelcol-isolation-forest 8888:8888 &
PF_METRICS_PID=$!
kubectl -n observability port-forward svc/otelcol-isolation-forest 13133:13133 &
PF_HEALTH_PID=$!

# Wait for port forwards to be ready
sleep 5

# Test health endpoint
echo "Testing health endpoint..."
curl -f http://localhost:13133/ && echo "✅ Health check passed" || echo "❌ Health check failed"

# Test metrics endpoint
echo "Testing metrics endpoint..."
curl -s http://localhost:8888/metrics | grep -q "otelcol_" && echo "✅ Metrics endpoint working" || echo "❌ Metrics endpoint failed"

# Test OTLP HTTP endpoint
echo "Testing OTLP HTTP endpoint..."
curl -f -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{"resourceMetrics":[]}' && echo "✅ OTLP HTTP endpoint working" || echo "❌ OTLP HTTP endpoint failed"

# Clean up port forwards
kill $PF_GRPC_PID $PF_HTTP_PID $PF_METRICS_PID $PF_HEALTH_PID 2>/dev/null || true

echo "Test complete!"
