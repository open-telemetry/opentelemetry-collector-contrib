// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package signingprocessor provides a processor that adds cryptographic
// integrity attributes to log records for use with the OpenTelemetry Audit
// Logging signal.
//
// For each log record it serializes the full record to RFC 8785 (JCS)
// canonical JSON and computes a signature using the configured JWA algorithm.
// The signature is stored as audit.integrity.value on the record.
// audit.integrity.algorithm and audit.integrity.certificate (for asymmetric
// algorithms) are set once per ResourceLogs block on the Resource attributes,
// after all records in the block have been signed successfully.
//
// Supported algorithms: RS256, RS512 (RSA PKCS#1 v1.5), ES256 (ECDSA P-256),
// EdDSA (Ed25519), and HMAC-SHA256.
//
// Key material is loaded at startup from one of four sources: local files,
// environment variables, a Kubernetes Secret, or an OpenBao/Vault KV secret.
// Each source supports both asymmetric key+certificate pairs and symmetric
// HMAC keys via dedicated configuration fields.
package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"
