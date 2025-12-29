// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage/internal/utils"

import (
	"crypto/sha256"
	"encoding/hex"
)

// Truncate ensures the filename is within filesystem limits.
// On most systems, the maximum file name length is 255 bytes.
// We use a SHA-256 hash to generate a fixed-length filename (64 characters).
func Truncate(name string) string {
	hashID := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hashID[:]) // filename safe
}
