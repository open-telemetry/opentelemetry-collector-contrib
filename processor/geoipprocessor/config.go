// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

const (
	providersKey = "providers"
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

// Config holds the configuration for the GeoIP processor.
type Config struct {
	// Providers specifies the sources to extract geographical information about a given IP.
	Providers map[string]provider.Config `mapstructure:"-"`

	// Context section allows specifying the source type to look for the IP. Available options: resource or record.
	Context ContextID `mapstructure:"context"`

	// An array of attribute names, which are used for the IP address lookup
	Attributes []attribute.Key `mapstructure:"attributes"`
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

func (cfg *Config) Validate() error {
	if len(cfg.Providers) == 0 {
		return errors.New("must specify at least one geo IP data provider when using the geoip processor")
	}

	// validate all provider's configuration
	for providerID, providerConfig := range cfg.Providers {
		if err := providerConfig.Validate(); err != nil {
			return fmt.Errorf("error validating provider %s: %w", providerID, err)
		}
	}

	if cfg.Attributes != nil && len(cfg.Attributes) == 0 {
		return errors.New("the attributes array must not be empty")
	}

	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused())
	if err != nil {
		return err
	}

	// dynamically load the individual providers configs based on the key name
	cfg.Providers = map[string]provider.Config{}

	// retrieve `providers` configuration section
	providersSection, err := componentParser.Sub(providersKey)
	if err != nil {
		return err
	}

	// loop through all defined providers and load their configuration
	for key := range providersSection.ToStringMap() {
		factory, ok := getProviderFactory(key)
		if !ok {
			return fmt.Errorf("invalid provider key: %s", key)
		}

		providerCfg := factory.CreateDefaultConfig()
		providerSection, err := providersSection.Sub(key)
		if err != nil {
			return err
		}
		err = providerSection.Unmarshal(providerCfg)
		if err != nil {
			return fmt.Errorf("error reading settings for provider type %q: %w", key, err)
		}

		cfg.Providers[key] = providerCfg
	}

	return nil
}
