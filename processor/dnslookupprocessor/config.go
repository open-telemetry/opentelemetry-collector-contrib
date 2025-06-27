// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"
import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
)

type ContextID string

const (
	resource ContextID = "resource"
	record   ContextID = "record"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
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
	Enabled bool `mapstructure:"enabled"`

	// Context specifies where to look for attributes (resource or record).
	Context ContextID `mapstructure:"context"`

	// SourceAttributes is a list of attributes to check for hostnames/IP. The first valid hostname/IP is used.
	SourceAttributes []string `mapstructure:"source_attributes"`

	// TargetAttribute is the attribute to store the resolved IP/hostname.
	TargetAttribute string `mapstructure:"target_attribute"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	validateLookupConfig := func(lc LookupConfig) error {
		if !lc.Enabled {
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
		return fmt.Errorf("resolve configuration: %w", err)
	}

	if err := validateLookupConfig(cfg.Reverse); err != nil {
		return fmt.Errorf("reverse configuration: %w", err)
	}

	return nil
}
