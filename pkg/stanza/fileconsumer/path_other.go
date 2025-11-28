// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"path/filepath"
)

// normalizePath cleans the path on non-Windows systems.
func normalizePath(path string) string {
	return filepath.Clean(path)
}
