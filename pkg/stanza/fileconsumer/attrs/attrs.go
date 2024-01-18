// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

import (
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	LogFileName         = "log.file.name"
	LogFilePath         = "log.file.path"
	LogFileNameResolved = "log.file.name_resolved"
	LogFilePathResolved = "log.file.path_resolved"
)

type Resolver struct {
	IncludeFileName         bool `mapstructure:"include_file_name,omitempty"`
	IncludeFilePath         bool `mapstructure:"include_file_path,omitempty"`
	IncludeFileNameResolved bool `mapstructure:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool `mapstructure:"include_file_path_resolved,omitempty"`
}

func (r *Resolver) Resolve(path string) (attributes map[string]any, err error) {
	// size 2 is sufficient if not resolving symlinks. This optimizes for the most performant cases.
	attributes = make(map[string]any, 2)
	if r.IncludeFileName {
		attributes[LogFileName] = filepath.Base(path)
	}
	if r.IncludeFilePath {
		attributes[LogFilePath] = path
	}
	if !r.IncludeFileNameResolved && !r.IncludeFilePathResolved {
		return attributes, nil
	}

	resolved := path
	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve symlinks: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return nil, fmt.Errorf("resolve abs: %w", err)
	}

	if r.IncludeFileNameResolved {
		attributes[LogFileNameResolved] = filepath.Base(abs)
	}
	if r.IncludeFilePathResolved {
		attributes[LogFilePathResolved] = abs
	}
	return attributes, nil
}
