// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package archive extracts collector executable updates from downloaded
// packages, dispatching on the configured archive format.
package archive

import (
	"context"
	"fmt"
	"os"
)

// maxAgentBytes is the maximum size of an agent binary the supervisor will write
// to disk during an install. It guards against unbounded writes.
const maxAgentBytes = 1 << 30 // 1 GiB

// Format is the format of the package downloaded by the supervisor. Additional
// formats (e.g. tar.gz) are added in later PRs; for now only the no-archive
// format (a raw binary) is supported.
type Format string

const (
	// FormatNone treats the downloaded package as a raw collector binary with no
	// archive wrapping.
	FormatNone Format = ""
)

// Extractor extracts the agent binary from a downloaded package.
type Extractor interface {
	// Extract writes the agent binary contained in pkg to destination.
	Extract(ctx context.Context, pkg []byte, destination string) error
}

// NewExtractor returns the Extractor for the given archive format, or an error if
// the format is not supported.
func NewExtractor(format Format) (Extractor, error) {
	switch format {
	case FormatNone:
		return rawExtractor{maxBytes: maxAgentBytes}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %q", string(format))
	}
}

var _ Extractor = rawExtractor{}

// rawExtractor treats the package bytes as a raw agent binary and writes them
// directly to destination.
type rawExtractor struct {
	// maxBytes is the maximum binary size the extractor will write. Inputs larger
	// than this are rejected rather than silently truncated.
	maxBytes int64
}

// Extract writes the raw bytes by creating or truncating the file at destination.
// It is the responsibility of the caller to ensure nothing of importance is overwritten.
// If an error occurs during write, the file at destination will be removed.
func (r rawExtractor) Extract(_ context.Context, pkg []byte, destination string) error {
	if int64(len(pkg)) > r.maxBytes {
		return fmt.Errorf("binary exceeds maximum size of %d bytes", r.maxBytes)
	}

	// Create or truncate the destination with executable permissions.
	if err := os.WriteFile(destination, pkg, 0o700); err != nil { //nolint:gosec // G306: the agent binary must be executable
		_ = os.Remove(destination)
		return fmt.Errorf("write binary to destination: %w", err)
	}

	return nil
}
