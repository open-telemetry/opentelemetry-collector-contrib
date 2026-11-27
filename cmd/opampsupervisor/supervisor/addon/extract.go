// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxAddonBytes bounds the total decompressed size of an addon package.
	maxAddonBytes = 1 << 30 // 1 GiB
	// maxAddonFiles bounds the number of entries in an addon package.
	maxAddonFiles = 10000
)

// ExtractTarGz extracts a gzipped tarball into destDir, which must already
// exist and should be empty. Unlike the archive package, which pulls a single
// named binary out of a package, an addon is a tree of files, so the whole
// archive is extracted.
//
// Entries that would resolve outside destDir are rejected rather than skipped:
// a package that tries to escape its directory is treated as hostile, not as
// partially usable. Symlinks and other non-regular entries are rejected for
// the same reason.
func ExtractTarGz(pkg []byte, destDir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(pkg))
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var (
		totalBytes int64
		fileCount  int
	)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tarball: %w", err)
		}

		fileCount++
		if fileCount > maxAddonFiles {
			return fmt.Errorf("addon package contains more than %d entries", maxAddonFiles)
		}

		target, err := resolveWithin(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return fmt.Errorf("create directory %q: %w", header.Name, err)
			}
		case tar.TypeReg:
			totalBytes += header.Size
			if totalBytes > maxAddonBytes {
				return fmt.Errorf("addon package exceeds maximum size of %d bytes", maxAddonBytes)
			}
			if err := writeFile(tarReader, target, header.FileInfo().Mode().Perm()); err != nil {
				return fmt.Errorf("extract %q: %w", header.Name, err)
			}
		default:
			// Symlinks, hardlinks, devices and FIFOs are all ways for a package to
			// reach outside its own directory or to surprise the Supervisor later.
			return fmt.Errorf("entry %q has unsupported type %q; addon packages may contain only regular files and directories",
				header.Name, string(rune(header.Typeflag)))
		}
	}

	return nil
}

// resolveWithin joins name onto dir and verifies the result stays inside dir.
func resolveWithin(dir, name string) (string, error) {
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("entry %q must not be an absolute path", name)
	}

	target := filepath.Join(dir, filepath.FromSlash(name))

	// filepath.Join cleans the result, so comparing prefixes is sufficient to
	// detect "..": a cleaned path that escapes dir cannot be prefixed by it.
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return "", fmt.Errorf("entry %q is not resolvable within the addon directory: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("entry %q escapes the addon directory", name)
	}

	return target, nil
}

// writeFile writes the current tar entry to target, creating parent directories
// as needed. Only the owner-executable bit is honored from the archive so that
// an entrypoint stays runnable without the package being able to widen access.
func writeFile(r io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	perm := os.FileMode(0o600)
	if mode&0o100 != 0 {
		perm = 0o700
	}

	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	// tar.Reader stops at the entry's declared size, so copying is bounded by the
	// per-entry accounting the caller already did.
	if _, err := io.Copy(f, r); err != nil {
		return err
	}

	return nil
}
