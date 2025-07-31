#!/bin/bash

echo "=== OpenTelemetry Collector Status ==="
kubectl -n observability get pods -l app=otelcol-isolation-forest

echo -e "\n=== Recent Logs ==="
kubectl -n observability logs -l app=otelcol-isolation-forest --tail=50

echo -e "\n=== Service Status ==="
kubectl -n observability get svc otelcol-isolation-forest

echo -e "\n=== ConfigMap ==="
kubectl -n observability get configmap otelcol-isolation-forest-config

echo -e "\n=== Events ==="
kubectl -n observability get events --field-selector involvedObject.name=otelcol-isolation-forest --sort-by='.lastTimestamp'

echo -e "\n=== Port Forward Commands ==="
echo "kubectl -n observability port-forward svc/otelcol-isolation-forest 4317:4317  # OTLP gRPC"
echo "kubectl -n observability port-forward svc/otelcol-isolation-forest 4318:4318  # OTLP HTTP"
echo "kubectl -n observability port-forward svc/otelcol-isolation-forest 8888:8888  # Metrics"
echo "kubectl -n observability port-forward svc/otelcol-isolation-forest 13133:13133  # Health"
