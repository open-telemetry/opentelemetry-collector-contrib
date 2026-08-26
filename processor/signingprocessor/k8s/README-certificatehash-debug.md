# Signing Processor - Deployment Guide

This guide explains how to deploy the signing processor with a debug exporter to a Kubernetes cluster.

## Overview

The signing processor adds cryptographic integrity verification to log records by:

- Computing a hash (SHA-256 or SHA-512) of each log record
- Signing the hash with an RSA private key using RSA PKCS1v15 (JWA: `RS256` or `RS512`)
- Adding the following attributes per the OTel Audit Logging spec:
  - `audit.integrity.value` — Base64-encoded RSA signature (record-level)
  - `audit.integrity.algorithm` — JWA algorithm identifier, e.g. `RS256` (resource-level)
  - `audit.integrity.certificate` — Certificate reference: `sha256:<hex>` fingerprint by default, or full Base64 DER if `certificate_ref: full` (resource-level)

## Prerequisites

- Kubernetes cluster (kind, minikube, or cloud cluster)
- kubectl configured
- Docker (for building custom image)
- Certificates (cert.pem, key.pem, ca.pem)

## Step 1: Build Custom Collector Image

The standard OpenTelemetry Collector image doesn't include custom processors. Build a custom image:

**Important:** Run this command from the repository root directory

```bash
docker build -f processor/signingprocessor/k8s/Dockerfile -t otelcol-signing:latest .
```

For kind clusters:

```bash
kind load docker-image otelcol-signing:latest --name otel-demo
```

See `BUILD-CUSTOM-IMAGE.md` for detailed instructions.

## Step 2: Generate Certificates

Generate test certificates using `openssl` or your preferred PKI tooling:

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/CN=signing-processor"
```

## Step 3: Create Kubernetes Secret

```bash
kubectl create namespace otel-demo
kubectl create secret generic otelcol-test-certs \
  --from-file=cert.pem=./cert.pem \
  --from-file=key.pem=./key.pem \
  --from-file=ca.pem=./ca.pem \
  -n otel-demo
```

## Step 4: Deploy Collector

```bash
kubectl apply -f processor/signingprocessor/k8s/otelcol-signing-debug.yaml
```

## Step 5: Restart Deployment

Restart the deployment to ensure pods pick up any configuration changes:

```bash
kubectl rollout restart deployment/otelcol-signing -n otel-demo
```

## Step 6: Verify Deployment

```bash
kubectl get pods -n otel-demo
kubectl logs -n otel-demo -l app=otelcol-signing
```

## Step 7: Test the Processor

Use the test script:

```bash
./processor/signingprocessor/k8s/test-send-log.sh
```

Or manually port-forward and send logs:

```bash
kubectl port-forward -n otel-demo service/otelcol-signing 4318:4318
```

Then send a test log using curl or the test script.

## Configuration

The processor configuration in the ConfigMap:

```yaml
processors:
  signing:
    hash_algorithm: SHA256  # or SHA512 — sets JWA algorithm to RS256 or RS512
    certificate_ref: fingerprint  # fingerprint (default) or full
    key_source:
      type: k8s_secret
      k8s_secret:
        name: otelcol-test-certs
        namespace: otel-demo
        cert_key: cert.pem
        key_key: key.pem
```

## Expected Output

When logs are processed, you should see in the collector logs (debug exporter output) that each log record has:

- `audit.integrity.value` - Base64-encoded RSA signature

## Troubleshooting

- **Pod crashes:** Check certificate paths and file permissions
- **Processor not found:** Ensure custom image is built and loaded
- **No hash/signature:** Check processor is in the pipeline and certificates are readable
