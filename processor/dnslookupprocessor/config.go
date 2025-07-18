// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"
import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	semconv "go.opentelemetry.io/otel/semconv/v1.31.0"
)

type lookupType string

type contextID string

const (
	resolve  lookupType = "resolve"
	reverse  lookupType = "reverse"
	resource contextID  = "resource"
	record   contextID  = "record"
)

func (l *lookupType) UnmarshalText(text []byte) error {
	str := lookupType(strings.ToLower(string(text)))
	switch str {
	case resolve, reverse:
		*l = str
		return nil
	default:
		return fmt.Errorf("unknown lookup type %s, available values: %s, %s", str, resolve, reverse)
	}
}

func (c *contextID) UnmarshalText(text []byte) error {
	str := contextID(strings.ToLower(string(text)))
	switch str {
	case resource, record:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %s, available values: %s, %s", str, resource, record)
	}
}

// Config holds the configuration for the DnsLookup processor.
type Config struct {
	Lookups []lookupConfig `mapstructure:"lookups"`

	// Hostfiles specifies the path to custom host files.
	Hostfiles []string `mapstructure:"hostfiles"`
}

// lookupConfig defines the configuration for forward/reverse DNS resolution.
type lookupConfig struct {
	// Type specifies the type of resolution (resolve or reverse).
	Type lookupType `mapstructure:"type"`

	// Context specifies where to look for attributes (resource or record).
	Context contextID `mapstructure:"context"`

	// SourceAttributes is a list of attributes to take hostnames/IP. The first valid hostname/IP is used.
	SourceAttributes []string `mapstructure:"source_attributes"`

	// TargetAttribute is the attribute to store the resolved IP/hostname.
	TargetAttribute string `mapstructure:"target_attribute"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	validateLookup := func(lc lookupConfig) error {
		if lc.Type != resolve && lc.Type != reverse {
			return fmt.Errorf("lookup type must be either 'resolve' or 'reverse', got: %s", lc.Type)
		}
		if len(lc.SourceAttributes) == 0 {
			return errors.New("at least one source_attributes must be specified for DNS resolution")
		}
		for _, attr := range lc.SourceAttributes {
			if attr == "" {
				return errors.New("source_attributes cannot contain empty strings")
			}
		}
		if lc.TargetAttribute == "" {
			return errors.New("target_attribute must be specified for DNS resolution")
		}
		if lc.Context != resource && lc.Context != record {
			return fmt.Errorf("lookup context must be either 'resource' or 'record', got: %s", lc.Context)
		}
		return nil
	}

	if len(cfg.Lookups) == 0 {
		return errors.New("at least one lookup must be configured")
	}

	for _, lookup := range cfg.Lookups {
		if err := validateLookup(lookup); err != nil {
			return err
		}
	}

	return nil
}

var _ confmap.Unmarshaler = (*Config)(nil)

func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if conf == nil {
		return nil
	}

	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}

	// Set default values for lookups
	for i := range cfg.Lookups {
		lookup := &cfg.Lookups[i]
		if lookup.Context == "" {
			lookup.Context = resource
		}
		if lookup.Type == resolve {
			if len(lookup.SourceAttributes) == 0 {
				lookup.SourceAttributes = []string{string(semconv.SourceAddressKey)}
			}
			if lookup.TargetAttribute == "" {
				lookup.TargetAttribute = sourceIPKey
			}
		}
		if lookup.Type == reverse {
			if len(lookup.SourceAttributes) == 0 {
				lookup.SourceAttributes = []string{sourceIPKey}
			}
			if lookup.TargetAttribute == "" {
				lookup.TargetAttribute = string(semconv.SourceAddressKey)
			}
		}
	}

	return nil
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{}
}
