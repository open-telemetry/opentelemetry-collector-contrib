package genainormalizerprocessor

import (
	"fmt"
)

// supportedProfiles lists all recognized profile names.
var supportedProfiles = map[string]bool{
	"openinference": true,
	"openllmetry":   true,
}

// supportedSemConvVersions lists all supported target semconv versions.
var supportedSemConvVersions = map[string]bool{
	"1.39.0": true,
}

// Config holds the configuration for the genainormalizer processor.
type Config struct {
	// SemConvVersion is the target OTel GenAI Semantic Convention version.
	// Supported: "1.39.0"
	SemConvVersion string `mapstructure:"semconv_version"`

	// Profiles to enable. Each profile maps a specific instrumentation
	// library's attributes to OTel GenAI Semantic Conventions.
	// Supported: openinference, openllmetry
	Profiles []string `mapstructure:"profiles"`

	// RemoveOriginals deletes source attributes after mapping.
	RemoveOriginals bool `mapstructure:"remove_originals"`

	// Overwrite controls whether existing target attributes are overwritten.
	// When false (default), if the target attribute already exists on the span,
	// the mapping is skipped.
	Overwrite bool `mapstructure:"overwrite"`

	// CustomMappings allows users to define additional source→target attribute mappings.
	// These are applied after profile mappings and override them on conflict.
	CustomMappings map[string]string `mapstructure:"custom_mappings"`
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.SemConvVersion != "" {
		if !supportedSemConvVersions[c.SemConvVersion] {
			return fmt.Errorf("unsupported semconv_version %q; supported: 1.39.0", c.SemConvVersion)
		}
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("at least one profile must be specified")
	}
	for _, p := range c.Profiles {
		if !supportedProfiles[p] {
			return fmt.Errorf("unknown profile %q; supported: openinference, openllmetry", p)
		}
	}
	return nil
}
