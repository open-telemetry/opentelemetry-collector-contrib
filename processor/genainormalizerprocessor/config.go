// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"errors"
	"fmt"
)

// SourceName identifies a supported source instrumentation convention.
type SourceName string

const (
	SourceOpenInference SourceName = "openinference"
	SourceOpenLLMetry   SourceName = "openllmetry"
)

var supportedSources = map[SourceName]struct{}{
	SourceOpenInference: {},
	SourceOpenLLMetry:   {},
}

// Source configures normalization behavior for a single source convention.
type Source struct {
	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite controls whether existing target attributes are overwritten.
	// When true, the target attribute is replaced if it already exists on the span.
	// When false (default), the mapping is skipped.
	Overwrite bool `mapstructure:"overwrite"`

	// CustomMappings defines additional source->target attribute mappings for this source.
	// Applied after built-in mappings and override them on conflict.
	CustomMappings map[string]string `mapstructure:"custom_mappings"`
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	// Sources selects which source conventions to normalize and their per-source options.
	Sources map[SourceName]Source `mapstructure:"sources"`
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("at least one source must be specified")
	}
	for name, src := range c.Sources {
		if _, ok := supportedSources[name]; !ok {
			return fmt.Errorf("unknown source %q", name)
		}
		for srcAttr, tgtAttr := range src.CustomMappings {
			if srcAttr == "" {
				return fmt.Errorf("sources[%q].custom_mappings: source attribute name must be non-empty", name)
			}
			if tgtAttr == "" {
				return fmt.Errorf("sources[%q].custom_mappings: target for %q must be non-empty", name, srcAttr)
			}
			if srcAttr == tgtAttr {
				return fmt.Errorf("sources[%q].custom_mappings: source and target are identical (%q)", name, srcAttr)
			}
		}
	}
	return nil
}
