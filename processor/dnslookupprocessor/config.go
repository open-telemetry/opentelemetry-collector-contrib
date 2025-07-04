// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"
import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	semconv "go.opentelemetry.io/otel/semconv/v1.31.0"
)

type contextID string

const (
	resource contextID = "resource"
	record   contextID = "record"
)

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
	// Resolve contains configuration for forward DNS lookups (hostname to IP).
	Resolve LookupConfig `mapstructure:"resolve"`

	// Reverse contains configuration for reverse DNS lookups (IP to hostname).
	Reverse LookupConfig `mapstructure:"reverse"`

	// Hostfiles specifies the path to custom host files.
	Hostfiles []string `mapstructure:"hostfiles"`
}

// LookupConfig defines the configuration for forward/reverse DNS resolution.
type LookupConfig struct {
	// Context specifies where to look for attributes (resource or record).
	Context contextID `mapstructure:"context"`

	// SourceAttributes is a list of attributes to check for hostnames/IP. The first valid hostname/IP is used.
	SourceAttributes []string `mapstructure:"source_attributes"`

	// TargetAttribute is the attribute to store the resolved IP/hostname.
	TargetAttribute string `mapstructure:"target_attribute"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	validateLookupConfig := func(lc LookupConfig) error {
		if reflect.DeepEqual(lc, LookupConfig{}) {
			return nil
		}

		if len(lc.SourceAttributes) == 0 {
			return errors.New("at least one source_attributes must be specified for DNS resolution")
		}
		if lc.TargetAttribute == "" {
			return errors.New("target_attribute must be specified for DNS resolution")
		}
		if lc.Context != resource && lc.Context != record {
			return fmt.Errorf("context must be either 'resource' or 'record', got: %s", lc.Context)
		}
		return nil
	}

	if err := validateLookupConfig(cfg.Resolve); err != nil {
		return fmt.Errorf("invalid resolve configuration: %w", err)
	}

	if err := validateLookupConfig(cfg.Reverse); err != nil {
		return fmt.Errorf("invalid reverse configuration: %w", err)
	}

	return nil
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}
	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	if !componentParser.IsSet("resolve") && !componentParser.IsSet("reverse") {
		return errors.New("at least one of 'resolve' or 'reverse' must be configured")
	}

	if componentParser.IsSet("resolve") {
		if !componentParser.IsSet("resolve::context") {
			cfg.Resolve.Context = resource
		}
		if !componentParser.IsSet("resolve::source_attributes") {
			cfg.Resolve.SourceAttributes = []string{string(semconv.SourceAddressKey)}
		}
		if !componentParser.IsSet("resolve::target_attribute") {
			cfg.Resolve.TargetAttribute = sourceIPKey
		}
	}

	if componentParser.IsSet("reverse") {
		if !componentParser.IsSet("reverse::context") {
			cfg.Reverse.Context = resource
		}
		if !componentParser.IsSet("reverse::source_attributes") {
			cfg.Reverse.SourceAttributes = []string{sourceIPKey}
		}
		if !componentParser.IsSet("reverse::target_attribute") {
			cfg.Reverse.TargetAttribute = string(semconv.SourceAddressKey)
		}
	}

	return nil
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{}
}
