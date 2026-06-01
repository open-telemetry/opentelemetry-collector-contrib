// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package archive installs collector executable updates from downloaded
// packages, dispatching on the configured archive format.
package archive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
		return rawInstaller{}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %q", string(format))
	}
}

var _ Installer = rawInstaller{}

// rawInstaller treats the package bytes as a raw agent binary and writes them
// directly to destination.
type rawInstaller struct{}

func (rawInstaller) Install(_ context.Context, pkg []byte, destination string) error {
	return writeBinaryToDestination(bytes.NewReader(pkg), destination)
}

// writeBinaryToDestination writes binary to destination, creating or truncating
// the file with executable permissions. At most maxAgentBytes are written.
func writeBinaryToDestination(binary io.Reader, destination string) error {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer f.Close()

	if _, err := io.CopyN(f, binary, maxAgentBytes); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("write binary to destination: %w", err)
	}

	return nil
}
