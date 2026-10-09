// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"slices"
	"strings"

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
// configuration. Credentials and metric mappings apply only to polling.
type PollConfig struct {
	ControllerConfig scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Endpoint is the SNMP target to request data from. Must be formatted as [udp|tcp|][4|6|]://{host}:{port}.
	// Default: udp://localhost:161
	// If no scheme is given, udp4 is assumed.
	// If no port is given, 161 is assumed.
	Endpoint string `mapstructure:"endpoint"`

	// Version is the version of SNMP to use for this connection.
	// Valid options: v1, v2c, v3.
	// Default: v2c
	Version string `mapstructure:"version"`

	// Community is the SNMP community string to use.
	// Only valid for versions "v1" and "v2c"
	// Default: public
	Community string `mapstructure:"community"`

	// User is the SNMP User for this connection.
	// Only valid for version “v3”
	User string `mapstructure:"user"`

	// SecurityLevel is the security level to use for this SNMP connection.
	// Only valid for version “v3”
	// Valid options: “no_auth_no_priv”, “auth_no_priv”, “auth_priv”
	// Default: "no_auth_no_priv"
	SecurityLevel string `mapstructure:"security_level"`

	// AuthType is the type of authentication protocol to use for this SNMP connection.
	// Only valid for version “v3” and if “no_auth_no_priv” is not selected for SecurityLevel
	// Valid options: “md5”, “sha”, “sha224”, “sha256”, “sha384”, “sha512”
	// Default: "md5"
	AuthType string `mapstructure:"auth_type"`

	// AuthPassword is the authentication password used for this SNMP connection.
	// Only valid for version "v3" and if "no_auth_no_priv" is not selected for SecurityLevel
	AuthPassword configopaque.String `mapstructure:"auth_password"`

	// PrivacyType is the type of privacy protocol to use for this SNMP connection.
	// Only valid for version “v3” and if "auth_priv" is selected for SecurityLevel
	// Valid options: “des”, “aes”, “aes192”, “aes256”, “aes192c”, “aes256c”
	// Default: "des"
	PrivacyType string `mapstructure:"privacy_type"`

	// PrivacyPassword is the authentication password used for this SNMP connection.
	// Only valid for version “v3” and if "auth_priv" is selected for SecurityLevel
	PrivacyPassword configopaque.String `mapstructure:"privacy_password"`

	// ResourceAttributes defines what resource attributes will be used for this receiver and is composed
	// of resource attribute names along with their resource attribute configurations
	ResourceAttributes map[string]*ResourceAttributeConfig `mapstructure:"resource_attributes"`

	// Attributes defines what attributes will be used on metrics for this receiver and is composed of
	// attribute names along with their attribute configurations
	Attributes map[string]*AttributeConfig `mapstructure:"attributes"`

	// Metrics defines what SNMP metrics will be collected for this receiver and is composed of metric
	// names along with their metric configurations
	Metrics map[string]*MetricConfig `mapstructure:"metrics"`
}

// validate checks the selected polling form. Keep it separate from confmap's
// recursive Validate interface so unused legacy defaults do not require metrics.
func (cfg *PollConfig) validate() error {
	var combinedErr error

	combinedErr = errors.Join(combinedErr, validateEndpoint(cfg))
	combinedErr = errors.Join(combinedErr, validateVersion(cfg))
	if strings.EqualFold(cfg.Version, "V3") {
		combinedErr = errors.Join(combinedErr, validateSecurity(cfg))
	}
	combinedErr = errors.Join(combinedErr, validateMetricConfigs(cfg))

	return combinedErr
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
			"collection_interval": cfg.LegacyPoll.ControllerConfig.CollectionInterval,
			"initial_delay":       cfg.LegacyPoll.ControllerConfig.InitialDelay,
			"timeout":             cfg.LegacyPoll.ControllerConfig.Timeout,
			"endpoint":            cfg.LegacyPoll.Endpoint,
			"version":             cfg.LegacyPoll.Version,
			"community":           cfg.LegacyPoll.Community,
			"user":                cfg.LegacyPoll.User,
			"security_level":      cfg.LegacyPoll.SecurityLevel,
			"auth_type":           cfg.LegacyPoll.AuthType,
			"auth_password":       cfg.LegacyPoll.AuthPassword,
			"privacy_type":        cfg.LegacyPoll.PrivacyType,
			"privacy_password":    cfg.LegacyPoll.PrivacyPassword,
			"resource_attributes": cfg.LegacyPoll.ResourceAttributes,
			"attributes":          cfg.LegacyPoll.Attributes,
			"metrics":             cfg.LegacyPoll.Metrics,
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
func (cfg *Config) effectivePollConfig() *PollConfig {
	selected := cfg.LegacyPoll
	if cfg.Poll != nil {
		selected = *cfg.Poll
	}
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
