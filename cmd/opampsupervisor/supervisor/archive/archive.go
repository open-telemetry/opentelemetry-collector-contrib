// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package archive installs collector executable updates from downloaded
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

// Installer installs the agent binary from a downloaded package.
type Installer interface {
	// Install writes the agent binary contained in pkg to destination.
	Install(ctx context.Context, pkg []byte, destination string) error
}

// NewInstaller returns the Installer for the given archive format, or an error if
// the format is not supported.
func NewInstaller(format Format) (Installer, error) {
	switch format {
	case FormatNone:
		return rawInstaller{maxBytes: maxAgentBytes}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %q", string(format))
	}
}

var _ Installer = rawInstaller{}

// rawInstaller treats the package bytes as a raw agent binary and writes them
// directly to destination.
type rawInstaller struct {
	// maxBytes is the maximum binary size the installer will write. Inputs larger
	// than this are rejected rather than silently truncated.
	maxBytes int64
}

func (r rawInstaller) Install(_ context.Context, pkg []byte, destination string) error {
	if int64(len(pkg)) > r.maxBytes {
		return fmt.Errorf("binary exceeds maximum size of %d bytes", r.maxBytes)
	}

	// Create or truncate the destination with executable permissions.
	if err := os.WriteFile(destination, pkg, 0o700); err != nil {
		return fmt.Errorf("write binary to destination: %w", err)
	}

	return nil
}
