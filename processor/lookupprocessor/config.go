// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
)

// ContextID specifies where to apply the lookup.
type ContextID string

const (
	ContextRecord   ContextID = "record"
	ContextResource ContextID = "resource"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case ContextRecord, ContextResource:
		*c = str
		return nil
	default:
		return fmt.Errorf("invalid context %q, must be one of: record, resource", str)
	}
}

type Config struct {
	Source SourceConfig `mapstructure:"source"`

	// Lookups defines the lookup rules.
	// Each rule specifies a key expression to extract the lookup value,
	// and one or more attribute mappings for where to write the results.
	Lookups []LookupConfig `mapstructure:"lookups"`
}

// SourceConfig captures the source type and its opaque settings.
//
// Source-specific fields are collected into Config via mapstructure's ",remain"
// tag and decoded later in the factory's createSource using a mapstructure
// decoder. An alternative would be to implement confmap.Unmarshaler on Config
// (like geoipprocessor does) to resolve the source config at unmarshal time.
// We defer decoding to the factory because the set of available source
// factories is not known until the factory is constructed (custom sources can
// be injected via WithSources), so the config layer cannot look up the correct
// factory. Both paths run during collector startup, so validation timing is
// equivalent in practice.
type SourceConfig struct {
	// Type is the source type identifier (e.g., "noop", "yaml").
	Type string `mapstructure:"type"`

	// Config holds the source-specific configuration as raw map.
	// It is decoded into the source's typed config struct by the factory
	// at processor creation time.
	Config map[string]any `mapstructure:",remain"`
}

// LookupConfig defines a single lookup rule.
type LookupConfig struct {
	// Key is an OTTL value expression for extracting the lookup key.
	// Examples: attributes["user.id"], Trim(attributes["raw.id"]),
	//           resource.attributes["service.name"]
	// Required.
	Key string `mapstructure:"key"`

	// Context is the default context for destination attributes.
	// Valid values: "record", "resource".
	// Default: "record"
	Context ContextID `mapstructure:"context"`

	// Attributes defines where to write lookup results.
	// Required: at least one mapping.
	Attributes []AttributeMapping `mapstructure:"attributes"`
}

// AttributeMapping defines where to write a lookup result.
type AttributeMapping struct {
	// Source is the field name in the lookup result for map results.
	// Leave empty for 1:1 scalar lookups.
	// Optional.
	Source string `mapstructure:"source"`

	// Destination is the attribute key to write the result to.
	// Required.
	Destination string `mapstructure:"destination"`

	// Default is the value to use when the lookup returns no result.
	// If not set and the lookup fails, no attribute is written.
	// Optional.
	Default string `mapstructure:"default"`

	// Context overrides the parent LookupConfig context for this mapping.
	// Valid values: "record", "resource".
	// Default: inherits from parent LookupConfig.
	Context ContextID `mapstructure:"context"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if len(cfg.Lookups) == 0 {
		return errors.New("at least one lookup must be configured")
	}

	for i, lookup := range cfg.Lookups {
		if lookup.Key == "" {
			return fmt.Errorf("lookups[%d]: key is required", i)
		}
		if len(lookup.Attributes) == 0 {
			return fmt.Errorf("lookups[%d]: at least one attribute mapping is required", i)
		}
		for j, attr := range lookup.Attributes {
			if attr.Destination == "" {
				return fmt.Errorf("lookups[%d].attributes[%d]: destination is required", i, j)
			}
		}
	}

	return nil
}

// GetContext returns the context for this lookup, defaulting to ContextRecord.
func (l *LookupConfig) GetContext() ContextID {
	if l.Context == "" {
		return ContextRecord
	}
	return l.Context
}

// GetContext returns the context for this mapping, falling back to the parent context.
func (m *AttributeMapping) GetContext(parent ContextID) ContextID {
	if m.Context == "" {
		return parent
	}
	return m.Context
}
