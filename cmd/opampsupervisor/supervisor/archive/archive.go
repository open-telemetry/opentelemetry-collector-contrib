// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package archive extracts collector executable updates from downloaded
// packages, dispatching on the archive format.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// maxAgentBytes is the maximum size of an agent binary the supervisor will write
// to disk during an install. It guards against unbounded writes.
const maxAgentBytes = 1 << 30 // 1 GiB

// Format is the format of the package downloaded by the supervisor.
type Format string

const (
	// FormatNone treats the downloaded package as a raw collector binary with no
	// archive wrapping.
	FormatNone Format = ""
	// FormatTarGzip treats the downloaded package as a gzipped tarball containing
	// the collector binary.
	FormatTarGzip Format = "tar.gz"
)

// Extractor extracts the agent binary from a downloaded package.
type Extractor interface {
	// Extract writes the agent binary contained in pkg to destination. binaryName
	// identifies which file inside the package is the agent binary for formats that
	// bundle multiple files; it is ignored for raw binaries.
	Extract(ctx context.Context, pkg []byte, binaryName, destination string) error
}

// NewExtractor returns the Extractor for the given archive format, or an error if
// the format is not supported.
func NewExtractor(format Format) (Extractor, error) {
	switch format {
	case FormatNone:
		return rawExtractor{maxBytes: maxAgentBytes}, nil
	case FormatTarGzip:
		return tarGzipExtractor{maxBytes: maxAgentBytes}, nil
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
func (r rawExtractor) Extract(_ context.Context, pkg []byte, _, destination string) error {
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

var _ Extractor = tarGzipExtractor{}

// tarGzipExtractor extracts the file named binaryName from a gzipped tarball and
// writes it to destination.
type tarGzipExtractor struct {
	// maxBytes is the maximum binary size the extractor will write. The size a
	// tar header declares is the decompressed size of its entry, and tar.Reader
	// returns at most that many bytes, so entries declaring more than maxBytes
	// are rejected before anything is written.
	maxBytes int64
}

func (e tarGzipExtractor) Extract(_ context.Context, pkg []byte, binaryName, destination string) error {
	if binaryName == "" {
		return errors.New("agent binary name is required for tar.gz archives")
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(pkg))
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			return fmt.Errorf("read tarball looking for %q: %w", binaryName, err)
		}
		if header.Name != binaryName {
			continue
		}
		if header.Size > e.maxBytes {
			return fmt.Errorf("binary exceeds maximum size of %d bytes", e.maxBytes)
		}
		break
	}

	if err := writeBinaryToDestination(tarReader, destination); err != nil {
		return fmt.Errorf("write binary to destination: %w", err)
	}

	return nil
}

// writeBinaryToDestination writes binary to destination, creating or truncating
// the file with executable permissions. If an error occurs during write, the
// file at destination will be removed.
func writeBinaryToDestination(binary io.Reader, destination string) error {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, binary); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("write binary to destination: %w", err)
	}

	return nil
}
