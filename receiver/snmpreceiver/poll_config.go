// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

var pollingConfigKeys = [...]string{
	"collection_interval", "initial_delay", "timeout", "endpoint", "version", "community",
	"user", "security_level", "auth_type", "auth_password", "privacy_type", "privacy_password",
	"resource_attributes", "attributes", "metrics",
}

var _ confmap.Marshaler = Config{}

// PollConfig groups the SNMP polling settings supported by the legacy top-level
// Config fields. Credentials and metric mappings apply only to polling.
type PollConfig struct {
	ControllerConfig scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Endpoint is the SNMP target. Defaults to udp://localhost:161.
	Endpoint string `mapstructure:"endpoint"`
	// Version is v1, v2c, or v3. Defaults to v2c.
	Version string `mapstructure:"version"`
	// Community is the v1/v2c community. Defaults to public.
	Community string `mapstructure:"community"`
	// User identifies the SNMPv3 user.
	User string `mapstructure:"user"`
	// SecurityLevel defaults to no_auth_no_priv.
	SecurityLevel string `mapstructure:"security_level"`
	// AuthType is the SNMPv3 authentication protocol. Defaults to MD5.
	AuthType string `mapstructure:"auth_type"`
	// AuthPassword is required for authenticated SNMPv3 polling.
	AuthPassword configopaque.String `mapstructure:"auth_password"`
	// PrivacyType is the SNMPv3 privacy protocol. Defaults to DES.
	PrivacyType string `mapstructure:"privacy_type"`
	// PrivacyPassword is required for private SNMPv3 polling.
	PrivacyPassword configopaque.String `mapstructure:"privacy_password"`
	// ResourceAttributes defines resource attributes used by polling metrics.
	ResourceAttributes map[string]*ResourceAttributeConfig `mapstructure:"resource_attributes"`
	// Attributes defines attributes used by polling metrics.
	Attributes map[string]*AttributeConfig `mapstructure:"attributes"`
	// Metrics defines the metric names and OID mappings to collect.
	Metrics map[string]*MetricConfig `mapstructure:"metrics"`
}

func defaultPollConfig() *PollConfig {
	return &PollConfig{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: defaultCollectionInterval,
			Timeout:            defaultTimeout,
		},
		Endpoint: defaultEndpoint, Version: defaultVersion, Community: defaultCommunity,
		SecurityLevel: defaultSecurityLevel, AuthType: defaultAuthType, PrivacyType: defaultPrivacyType,
	}
}

// Marshal emits only the selected polling form. Keep the original typed values
// so the enclosing confmap encoder applies its redaction option to credentials.
func (cfg Config) Marshal(conf *confmap.Conf) error {
	input := map[string]any{"poll": cfg.Poll}
	if cfg.Poll == nil {
		input = map[string]any{
			"collection_interval": cfg.ControllerConfig.CollectionInterval,
			"initial_delay":       cfg.ControllerConfig.InitialDelay,
			"timeout":             cfg.ControllerConfig.Timeout,
			"endpoint":            cfg.Endpoint,
			"version":             cfg.Version,
			"community":           cfg.Community,
			"user":                cfg.User,
			"security_level":      cfg.SecurityLevel,
			"auth_type":           cfg.AuthType,
			"auth_password":       cfg.AuthPassword,
			"privacy_type":        cfg.PrivacyType,
			"privacy_password":    cfg.PrivacyPassword,
			"resource_attributes": cfg.ResourceAttributes,
			"attributes":          cfg.Attributes,
			"metrics":             cfg.Metrics,
		}
	}
	return conf.Merge(confmap.NewFromStringMap(input))
}

func (cfg *Config) unmarshalPollConfig(conf *confmap.Conf) error {
	input := conf.ToStringMap()
	pollValue, present := input["poll"]
	if !present {
		return nil
	}
	if pollValue == nil {
		return errors.New("poll must be a non-null mapping")
	}
	pollMap, ok := pollValue.(map[string]any)
	if !ok {
		return errors.New("poll must be a mapping")
	}
	if pollMap == nil {
		return errors.New("poll must be a non-null mapping")
	}
	// Use key existence rather than nonzero decoded values: explicit null,
	// false, empty, and default-valued legacy options are still conflicts.
	for _, legacy := range pollingConfigKeys {
		if _, exists := input[legacy]; exists {
			return fmt.Errorf("poll cannot be combined with top-level polling option %q", legacy)
		}
	}
	cfg.Poll = defaultPollConfig()
	return nil
}

// effectivePollConfig selects one polling form for existing scraper/client code.
// Defaults and scraper normalization mutate metric definitions, OID slices,
// and attribute mappings. Copy their containers and pointed-to configs so they
// cannot change the source configuration. Read-only OID association slices and
// enum values can retain their references.
func (cfg *Config) effectivePollConfig() *Config {
	selected := *cfg
	if cfg.Poll != nil {
		poll := cfg.Poll
		selected = Config{
			ControllerConfig: poll.ControllerConfig,
			Endpoint:         poll.Endpoint, Version: poll.Version, Community: poll.Community,
			User: poll.User, SecurityLevel: poll.SecurityLevel,
			AuthType: poll.AuthType, AuthPassword: poll.AuthPassword,
			PrivacyType: poll.PrivacyType, PrivacyPassword: poll.PrivacyPassword,
			ResourceAttributes: poll.ResourceAttributes, Attributes: poll.Attributes, Metrics: poll.Metrics,
		}
	}
	selected.Poll = nil
	if selected.Metrics != nil {
		copied := make(map[string]*MetricConfig, len(selected.Metrics))
		for name, metric := range selected.Metrics {
			if metric == nil {
				copied[name] = nil
				continue
			}
			metricCopy := *metric
			metricCopy.ScalarOIDs = slices.Clone(metric.ScalarOIDs)
			metricCopy.ColumnOIDs = slices.Clone(metric.ColumnOIDs)
			if metric.Gauge != nil {
				gaugeCopy := *metric.Gauge
				metricCopy.Gauge = &gaugeCopy
			}
			if metric.Sum != nil {
				sumCopy := *metric.Sum
				metricCopy.Sum = &sumCopy
			}
			copied[name] = &metricCopy
		}
		selected.Metrics = copied
	}
	if selected.Attributes != nil {
		copied := make(map[string]*AttributeConfig, len(selected.Attributes))
		for name, attribute := range selected.Attributes {
			if attribute == nil {
				copied[name] = nil
				continue
			}
			attributeCopy := *attribute
			copied[name] = &attributeCopy
		}
		selected.Attributes = copied
	}
	if selected.ResourceAttributes != nil {
		copied := make(map[string]*ResourceAttributeConfig, len(selected.ResourceAttributes))
		for name, attribute := range selected.ResourceAttributes {
			if attribute == nil {
				copied[name] = nil
				continue
			}
			attributeCopy := *attribute
			copied[name] = &attributeCopy
		}
		selected.ResourceAttributes = copied
	}
	return &selected
}
