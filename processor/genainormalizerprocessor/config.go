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
	// SourceOpenLLMetry enables normalization of OpenLLMetry (Traceloop) attributes.
	SourceOpenLLMetry SourceName = "openllmetry"
	// SourceCustom enables user-defined attribute and value mappings.
	SourceCustom SourceName = "custom"
)

var supportedSources = map[SourceName]struct{}{
	SourceOpenInference: {},
	SourceOpenLLMetry:   {},
	SourceCustom:        {},
}

// Source configures normalization behavior for a single source convention.
type Source struct {
	_ struct{} // required by componenttest.CheckConfigStruct: forces keyed literals

	// Name identifies the source convention (e.g. "openinference").
	Name SourceName `mapstructure:"name"`

	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite replaces target attributes that already exist on the span.
	// When false (default), existing target attributes are left unchanged.
	Overwrite bool `mapstructure:"overwrite"`

	// Mappings is the source-attribute -> target-attribute rename table for
	// the "custom" source. Required when Name == SourceCustom and rejected
	// for any other source.
	Mappings map[string]string `mapstructure:"mappings"`

	// ValueMappings is keyed by the post-rename target attribute name and
	// folds source string values onto preferred target string values.
	// Lookups are case-insensitive on the source value. Only valid when
	// Name == SourceCustom; each key must appear as a target in Mappings.
	ValueMappings map[string]map[string]string `mapstructure:"value_mappings"`
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	_ struct{} // required by componenttest.CheckConfigStruct: forces keyed literals

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
	// Duplicate-detection applies to built-in sources only; multiple
	// custom blocks are allowed.
	seen := make(map[SourceName]struct{}, len(c.Sources))
	for i, src := range c.Sources {
		if _, ok := supportedSources[src.Name]; !ok {
			return fmt.Errorf("sources[%d]: unknown source %q", i, src.Name)
		}
		if src.Name == SourceCustom {
			if len(src.Mappings) == 0 {
				return fmt.Errorf("sources[%d]: custom source requires non-empty %q", i, "mappings")
			}
			targets := make(map[string]struct{}, len(src.Mappings))
			for _, t := range src.Mappings {
				targets[t] = struct{}{}
			}
			for k := range src.ValueMappings {
				if _, ok := targets[k]; !ok {
					return fmt.Errorf("sources[%d]: %q key %q is not a target in %q", i, "value_mappings", k, "mappings")
				}
			}
		} else {
			if len(src.Mappings) > 0 {
				return fmt.Errorf("sources[%d]: %q is only valid for the %q source", i, "mappings", SourceCustom)
			}
			if len(src.ValueMappings) > 0 {
				return fmt.Errorf("sources[%d]: %q is only valid for the %q source", i, "value_mappings", SourceCustom)
			}
			if _, dup := seen[src.Name]; dup {
				return fmt.Errorf("sources[%d]: duplicate source %q", i, src.Name)
			}
			seen[src.Name] = struct{}{}
		}
	}
	return nil
}
