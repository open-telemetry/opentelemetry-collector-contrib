// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// supportedProfiles lists all recognized profile names.
var supportedProfiles = map[string]struct{}{
	"openinference": {},
	"openllmetry":   {},
}

// sortedProfileNames returns the supported profile names in sorted order.
func sortedProfileNames() string {
	names := make([]string, 0, len(supportedProfiles))
	for k := range supportedProfiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	// Profiles to enable. Each profile maps a specific instrumentation
	// library's attributes to OTel GenAI Semantic Conventions.
	Profiles []string `mapstructure:"profiles"`

	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite controls whether existing target attributes are overwritten.
	// When false (default), if the target attribute already exists on the span,
	// the mapping is skipped.
	Overwrite bool `mapstructure:"overwrite"`

	// CustomMappings allows users to define additional source->target attribute mappings.
	// These are applied after profile mappings and override them on conflict.
	CustomMappings map[string]string `mapstructure:"custom_mappings"`
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile must be specified")
	}
	for _, p := range c.Profiles {
		if _, ok := supportedProfiles[p]; !ok {
			return fmt.Errorf("unknown profile %q; supported: %s", p, sortedProfileNames())
		}
	}
	for src, tgt := range c.CustomMappings {
		if src == "" {
			return errors.New("custom_mappings: source attribute name must be non-empty")
		}
		if tgt == "" {
			return fmt.Errorf("custom_mappings: target for %q must be non-empty", src)
		}
		if src == tgt {
			return fmt.Errorf("custom_mappings: source and target are identical (%q)", src)
		}
	}
	return nil
}
