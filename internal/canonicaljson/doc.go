// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package canonicaljson provides RFC 8785 (JCS) canonical JSON serialization
// for OpenTelemetry log records.
//
// This package is used internally by the signing and verify processors to
// produce deterministic JSON representations of log records for cryptographic
// signing and verification.
//
// The serialization excludes certain fields that should not be part of the
// signed payload (such as the signature itself) and validates that all strings
// contain valid UTF-8 to prevent canonicalization issues.
package canonicaljson // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"
