// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"errors"
	"time"
)

// FileSourceConfig holds configuration common to file-backed lookup sources.
// Embed it with `mapstructure:",squash"` so that `path` and `reload_interval`
// remain top-level keys in the source's configuration.
type FileSourceConfig struct {
	// Path is the path to the lookup file. Required.
	Path string `mapstructure:"path"`

	// ReloadInterval, when > 0, re-reads the file on this interval so changes
	// take effect without a collector restart. 0 disables reloading.
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

// Validate checks the shared file-source fields. Embedding sources call this
// from their own Validate() before their source-specific checks.
func (c FileSourceConfig) Validate() error {
	if c.Path == "" {
		return errors.New("path is required")
	}
	if c.ReloadInterval < 0 {
		return errors.New("reload_interval must not be negative")
	}
	return nil
}
