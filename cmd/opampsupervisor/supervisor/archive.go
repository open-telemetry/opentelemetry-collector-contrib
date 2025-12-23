// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// maxAgentBytes is the max size of an agent package that will be accepted.
// It is 1 gibibyte.
const maxAgentBytes = 1024 * 1024 * 1024

// installFunc is a function that unpacks an archive and writes the binary to the given destination.
type installFunc func(ctx context.Context, archive []byte, binaryName, destination string) error

// NewInstallFunc creates a new installFunc based on the archive format.
func NewInstallFunc(archiveFormat config.Archive) (installFunc, error) {
	switch archiveFormat {
	case config.ArchiveTarGzip:
		return tarGzipInstall, nil
	case config.ArchiveDefault:
		return defaultInstall, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", archiveFormat)
	}
}

// writeBinaryToDestination is a common function that writes the given binary to the given destination.
func writeBinaryToDestination(binary io.Reader, destination string) error {
	// create the destination file
	agentFile, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE, 0o700)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer agentFile.Close()

	// write the binary to the destination file
	if _, err = io.CopyN(agentFile, binary, maxAgentBytes); err != nil {
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("copy binary to destination file: %w", err)
		}
	}
	return nil
}

// Default Install Func /////////////////////////////////////////////////////////////////////////

var _ installFunc = defaultInstall

// defaultInstall installs an archive with no specific format.
// It simply writes the binary to the destination file.
func defaultInstall(_ context.Context, archive []byte, _, destination string) error {
	if err := writeBinaryToDestination(bytes.NewReader(archive), destination); err != nil {
		return fmt.Errorf("write binary to destination file: %w", err)
	}
	return nil
}

// Tarball Install Func /////////////////////////////////////////////////////////////////////////

var _ installFunc = tarGzipInstall

// tarGzipInstall installs a tarball archive.
// Creates a tar reader based on a gzip reader, verifies the binary name exists in the tarball,
// and extracts the binary to the given destination.
func tarGzipInstall(_ context.Context, archive []byte, binaryName, destination string) error {
	// create gzip reader
	gzipReader, err := gzip.NewReader(bytes.NewBuffer(archive))
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// wrap gzip reader in a tar reader
	tar := tar.NewReader(gzipReader)
	h, err := tar.Next()
	if err != nil {
		return fmt.Errorf("first tarball read for collector: %w", err)
	}

	// verify the binary name exists in the tarball
	for h.Name != binaryName {
		h, err = tar.Next()
		if err != nil {
			return fmt.Errorf("read tarball for collector: %w", err)
		}
	}

	// write the binary to the destination file
	if err = writeBinaryToDestination(tar, destination); err != nil {
		return fmt.Errorf("write binary to destination file: %w", err)
	}

	return nil
}

// TODO: Add support for other archive formats below.
