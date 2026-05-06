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
	// SourceOpenLLMetry enables normalization of OpenLLMetry attributes.
	SourceOpenLLMetry SourceName = "openllmetry"
	// SourceCustom applies only user-supplied mappings via CustomMappings.
	SourceCustom SourceName = "custom"
)

var supportedSources = map[SourceName]struct{}{
	SourceOpenInference: {},
	SourceOpenLLMetry:   {},
	SourceCustom:        {},
}

// Source configures normalization behavior for a single source convention.
type Source struct {
	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite replaces target attributes that already exist on the span.
	// When false (default), existing target attributes are left unchanged.
	Overwrite bool `mapstructure:"overwrite"`

	// CustomMappings defines additional source->target attribute mappings.
	// Applied after built-in mappings and override them on conflict.
	// Required when the source is SourceCustom.
	CustomMappings map[string]string `mapstructure:"custom_mappings"`
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	// Sources selects which source conventions to normalize and their per-source options.
	// At least one source must be specified.
	Sources map[SourceName]Source `mapstructure:"sources"`
}

var _ xconfmap.Validator = (*Config)(nil)

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("at least one source must be specified")
	}
	for name, src := range c.Sources {
		if _, ok := supportedSources[name]; !ok {
			return fmt.Errorf("unknown source %q", name)
		}
		if name == SourceCustom && len(src.CustomMappings) == 0 {
			return fmt.Errorf("sources[%q]: custom_mappings must be non-empty for the custom source", name)
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
