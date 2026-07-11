// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/confmap/xconfmap"
)

// SourceName identifies a source instrumentation convention. Built-in
// names (e.g. "openinference") get pre-defined mapping tables; any other
// name is a user-defined source whose mappings come from the config.
type SourceName string

const (
	// SourceOpenInference enables normalization of OpenInference attributes.
	SourceOpenInference SourceName = "openinference"
	// SourceOpenLLMetry enables normalization of OpenLLMetry (Traceloop) attributes.
	SourceOpenLLMetry SourceName = "openllmetry"
)

var builtInSources = map[SourceName]struct{}{
	SourceOpenInference: {},
	SourceOpenLLMetry:   {},
}

// Source configures normalization behavior for a single source convention.
type Source struct {
	_ struct{}

	// Name identifies the source. Built-in names (e.g. "openinference",
	// "openllmetry") use pre-defined mapping tables; any other name is a
	// user-defined source whose mappings come from this entry's Mappings
	// and ValueMappings fields.
	Name SourceName `mapstructure:"name"`

	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite replaces target attributes that already exist on the span.
	// When false (default), existing target attributes are left unchanged.
	Overwrite bool `mapstructure:"overwrite"`

	// Mappings is the source-attribute -> target-attribute rename table.
	// Required for user-defined sources; rejected on built-in sources.
	Mappings map[string]string `mapstructure:"mappings"`

	// ValueMappings is keyed by the post-rename target attribute name and
	// folds source string values onto preferred target string values.
	// Source-value lookups are exact-match. Only valid on user-defined
	// sources; each key must appear as a target in Mappings.
	ValueMappings map[string]map[string]string `mapstructure:"value_mappings"`
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	_ struct{}

	// Sources is an ordered list of sources to normalize. Each span is
	// processed by every source in the order specified. At least one source
	// must be specified.
	Sources []Source `mapstructure:"sources"`
}

var _ xconfmap.Validator = (*Config)(nil)

// builtInSourceNames returns a sorted comma-separated list of built-in
// source names for use in error messages.
func builtInSourceNames() string {
	names := make([]string, 0, len(builtInSources))
	for name := range builtInSources {
		names = append(names, string(name))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("at least one source must be specified")
	}
	seen := make(map[SourceName]struct{}, len(c.Sources))
	for i, src := range c.Sources {
		if src.Name == "" {
			return fmt.Errorf("sources[%d]: %q must be set", i, "name")
		}
		if _, dup := seen[src.Name]; dup {
			return fmt.Errorf("sources[%d]: duplicate source %q", i, src.Name)
		}
		seen[src.Name] = struct{}{}

		if _, builtIn := builtInSources[src.Name]; builtIn {
			if len(src.Mappings) > 0 {
				return fmt.Errorf("sources[%d]: %q is not valid on built-in source %q", i, "mappings", src.Name)
			}
			if len(src.ValueMappings) > 0 {
				return fmt.Errorf("sources[%d]: %q is not valid on built-in source %q", i, "value_mappings", src.Name)
			}
			continue
		}

		// User-defined source.
		if len(src.Mappings) == 0 {
			return fmt.Errorf("sources[%d]: user-defined source %q requires non-empty %q (built-in sources, which do not require mappings: %s)", i, src.Name, "mappings", builtInSourceNames())
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
	}
	return nil
}
