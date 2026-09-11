// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"os"
	"path/filepath"
)

// normalizePath cleans the path on non-Windows systems.
// Returns the cleaned path and false (no corruption detection on non-Windows).
func normalizePath(path string) (string, bool) {
	return filepath.Clean(path), false
}

func openFile(path string) (*os.File, error) {
	return os.Open(path) // #nosec - operator must read in files defined by user
}
