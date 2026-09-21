// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

// Package certificatelogverifyprocessor provides a processor that verifies
// cryptographic integrity attributes on log records for use with the
// OpenTelemetry Audit Logging signal.
//
// For each log record it serializes the record to RFC 8785 (JCS) canonical
// JSON and verifies audit.integrity.value using the configured HMAC key
// and/or certificate public key. Optionally it validates hash-chain
// continuity and can persist verification failures to a dead-letter store.
//
// Supported integrity algorithms mirror those produced by signingprocessor:
// RS256, RS512, ES256, EdDSA, and HMAC-SHA256.
//
// Key material is loaded at startup from a local file or a Kubernetes Secret.
package certificatelogverifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/certificatelogverifyprocessor"
