// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/confmap/xconfmap"
)

// SourceName identifies a supported source instrumentation convention.
type SourceName string

const (
	// SourceOpenInference enables normalization of OpenInference attributes.
	SourceOpenInference SourceName = "openinference"
)

var supportedSources = map[SourceName]struct{}{
	SourceOpenInference: {},
}

// Source configures normalization behavior for a single source convention.
type Source struct {
	_ struct{} // prevent unkeyed literals

	// Name identifies the source convention (e.g. "openinference").
	Name SourceName `mapstructure:"name"`

	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite replaces target attributes that already exist on the span.
	// When false (default), existing target attributes are left unchanged.
	Overwrite bool `mapstructure:"overwrite"`
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	_ struct{} // prevent unkeyed literals

	// Sources is an ordered list of sources to normalize. Each span is
	// processed by every source in the order specified. At least one source
	// must be specified.
	Sources []Source `mapstructure:"sources"`
}

var _ xconfmap.Validator = (*Config)(nil)

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("at least one source must be specified")
	}
	seen := make(map[SourceName]struct{}, len(c.Sources))
	for i, src := range c.Sources {
		if _, ok := supportedSources[src.Name]; !ok {
			return fmt.Errorf("sources[%d]: unknown source %q", i, src.Name)
		}
		if _, dup := seen[src.Name]; dup {
			return fmt.Errorf("sources[%d]: duplicate source %q", i, src.Name)
		}
		seen[src.Name] = struct{}{}
	}
	return nil
}
