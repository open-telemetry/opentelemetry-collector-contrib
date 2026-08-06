# Verify Signed Log Script

This script verifies the integrity and authenticity of log records that have been processed by the signing processor.

## Overview

The verification script:

1. Reconstructs the original log record (without signature attribute)
2. Serializes it the same way the processor does
3. Computes the hash and verifies the RSA signature against it

## Prerequisites

- Go compiler (for building the verification tool)
- Certificate file (cert.pem) with the public key
- Log file in JSON format (OTLP format or single log record)
- `jq` (for the shell helper scripts)
- kubectl configured (if extracting certificates from Kubernetes)

## Extracting Certificates from Kubernetes

Before verifying logs, you may need to extract the certificate from the Kubernetes secret:

```bash
# Extract all certificates (cert.pem, key.pem, ca.pem)
./extract-cert.sh

# Extract only the certificate (for verification)
./extract-cert.sh --cert-only

# Extract to a specific directory
./extract-cert.sh --output-dir ./certs

# Extract from a different namespace/secret
./extract-cert.sh --namespace my-namespace --secret my-secret --cert-only

# Extract from OpenBao instead of Kubernetes
./extract-cert.sh --source openbao --bao-addr https://bao.internal:8200 \
  --bao-path secret/data/signing/cert --bao-token s.xxxx
```

## Usage

### Shell Script

```bash
# Verify log from file with certificate file
./verify-signed-log.sh --log log.json --cert cert.pem

# Verify with SHA512 algorithm
./verify-signed-log.sh --log log.json --cert cert.pem --hash SHA512

# Verify with verbose output
./verify-signed-log.sh --log log.json --cert cert.pem --verbose

# Fetch certificate from Kubernetes secret and verify
./verify-signed-log.sh --log log.json --from-k8s --namespace otel-demo --secret otelcol-test-certs
```

### Go Tool Directly

```bash
# Build the tool
go build -o verify-signed-log verify-signed-log.go

# Verify log
./verify-signed-log -log log.json -cert cert.pem

# Verify with SHA512
./verify-signed-log -log log.json -cert cert.pem -hash SHA512

# Verify with verbose output
./verify-signed-log -log log.json -cert cert.pem -verbose

# Read from stdin
cat log.json | ./verify-signed-log -log - -cert cert.pem
```

## Log File Format

The script accepts log files in two formats:

### OTLP Format (from collector debug exporter)

```json
{
  "resourceLogs": [
    {
      "scopeLogs": [
        {
          "logRecords": [
            {
              "body": "Test log message",
              "attributes": {
                "audit.integrity.value": "base64-encoded-signature",
                "other.attribute": "value"
              },
              "timestamp": 1234567890000000000,
              "severity_number": 9,
              "severity_text": "INFO"
            }
          ]
        }
      ]
    }
  ]
}
```

### Single Log Record Format

```json
{
  "body": "Test log message",
  "attributes": {
    "audit.integrity.value": "base64-encoded-signature",
    "other.attribute": "value"
  },
  "timestamp": 1234567890000000000,
  "severity_number": 9,
  "severity_text": "INFO"
}
```

## Output

The script will output:

- ✅ Success message for each verified log record
- ❌ Error messages for failed verifications
- Detailed information when using `-verbose` flag

Example output:

```
✅ Log record 1: Signature verified successfully
✅ Log record 2: Signature verified successfully

✅ All log records verified successfully!
```

## How It Works

1. **Signature Verification**: The script reconstructs the log record exactly as it was when signed by the processor (excluding `audit.integrity.*` attributes), serializes it to canonical JSON (RFC 8785 JCS), computes the hash, then verifies the RSA PKCS1v15 signature from `audit.integrity.value` against that hash using the public key from the certificate.

2. **Algorithm**: The signing algorithm is indicated by the `audit.integrity.algorithm` resource attribute (`RS256` for SHA-256, `RS512` for SHA-512).

3. **Certificate Reference**: The `audit.integrity.certificate` resource attribute identifies the signing certificate. By default the processor writes a `sha256:<hex>` fingerprint; if configured with `certificate_ref: full` it writes the full Base64-encoded DER certificate instead.

## Troubleshooting

### Signature Verification Failed

- Ensure you're using the correct certificate (the one matching the private key used for signing)
- Check that the certificate file is in PEM format
- Verify the certificate contains an RSA public key

### Certificate Errors

- Ensure the certificate file path is correct
- Check that the certificate is in PEM format (starts with `-----BEGIN CERTIFICATE-----`)
- Verify the certificate hasn't been corrupted

## Integration with Collector Logs

To verify logs from the collector's debug exporter:

```bash
# Step 1: Extract certificate from Kubernetes
./extract-cert.sh --cert-only

# Step 2: Get logs from collector
kubectl logs -n otel-demo -l app=otelcol-signing --tail=100 > collector-logs.txt

# Step 3: Extract JSON log records (you may need to parse the collector output)
# Then verify with the script
./verify-signed-log.sh --log extracted-log.json --cert cert.pem
```

Or use `--from-k8s` to automatically fetch the certificate:

```bash
# Verify logs and fetch certificate from K8s automatically
./verify-signed-log.sh --log extracted-log.json --from-k8s
```

Or run the full end-to-end test (send a log, capture it, verify it):

```bash
./test-and-verify.sh
```

## Notes

- The script verifies each log record independently
- Multiple log records can be verified in a single run
- The script exits with code 0 if all records verify successfully, otherwise exits with code 1
