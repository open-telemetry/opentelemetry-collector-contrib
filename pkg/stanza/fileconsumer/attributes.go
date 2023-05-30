// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"path/filepath"
	"runtime"

	"go.uber.org/multierr"
)

type FileAttributes struct {
	Name             string `json:"-"`
	Path             string `json:"-"`
	NameResolved     string `json:"-"`
	PathResolved     string `json:"-"`
	HeaderAttributes map[string]any
}

// HeaderAttributesCopy gives a copy of the HeaderAttributes, in order to restrict mutation of the HeaderAttributes.
func (f *FileAttributes) HeaderAttributesCopy() map[string]any {
	return mapCopy(f.HeaderAttributes)
}

// resolveFileAttributes resolves file attributes
// and sets it to empty string in case of error
func resolveFileAttributes(path string) (*FileAttributes, error) {
	resolved := ""
	var symErr error
	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, symErr = filepath.EvalSymlinks(path)
	} else {
		resolved = path
	}
	abs, absErr := filepath.Abs(resolved)

	return &FileAttributes{
		Path:         path,
		Name:         filepath.Base(path),
		PathResolved: abs,
		NameResolved: filepath.Base(abs),
	}, multierr.Combine(symErr, absErr)
}
