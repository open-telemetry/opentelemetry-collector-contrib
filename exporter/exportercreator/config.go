// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"fmt"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	// exportersConfigKey is the config key name used to specify the subexporters.
	exportersConfigKey = "exporters"
	// configKey is the key name in a subexporter.
	configKey = "config"
)

// exporterConfig describes an exporter instance with a default config.
type exporterConfig struct {
	// id is the id of the subexporter (ie <exporter type>/<id>).
	id component.ID
	// config is the map configured by the user in the config file. It is the contents of the map from
	// the "config" section. The keys and values are arbitrarily configured by the user.
	config     userConfigMap
	endpointID observer.EndpointID
}

// userConfigMap is an arbitrary map of string keys to arbitrary values as specified by the user
type userConfigMap map[string]any

type exporterSignals struct {
	metrics bool
	logs    bool
	traces  bool
}

// exporterTemplate is the configuration of a single subexporter.
type exporterTemplate struct {
	exporterConfig

	// Rule is the discovery rule that when matched will create an exporter instance
	// based on exporterTemplate.
	Rule string `mapstructure:"rule"`
	// ResourceAttributes is a map of resource attributes to associate with this exporter's endpoint.
	// It can contain expr expressions for endpoint env value expansion.
	ResourceAttributes map[string]any `mapstructure:"resource_attributes"`
	// Signals specifies which signal types (logs, metrics, traces) this exporter should handle.
	// If not specified, all signals are enabled by default.
	Signals exporterSignals `mapstructure:"signals"`
	rule    rule
	signals exporterSignals
}

// newExporterTemplate creates an exporterTemplate instance from the full name of a subexporter
// and its arbitrary config map values.
func newExporterTemplate(name string, cfg userConfigMap) (exporterTemplate, error) {
	id := component.ID{}
	if err := id.UnmarshalText([]byte(name)); err != nil {
		return exporterTemplate{}, err
	}

	return exporterTemplate{
		signals: exporterSignals{metrics: true, logs: true, traces: true},
		exporterConfig: exporterConfig{
			id:         id,
			config:     cfg,
			endpointID: observer.EndpointID("endpoint.id"),
		},
	}, nil
}

var _ confmap.Unmarshaler = (*Config)(nil)

// RoutingRule defines a mapping between a resource attribute and an endpoint property.
type RoutingRule struct {
	// ResourceAttribute is the resource attribute key to match (e.g., "k8s.pod.labels.app")
	ResourceAttribute string `mapstructure:"resource_attribute"`
	// EndpointProperty is the endpoint property to match against (e.g., "labels.app", "spec.region")
	// Supports dot notation for nested fields
	EndpointProperty string `mapstructure:"endpoint_property"`
}

// RoutingConfig defines the routing configuration for directing telemetry to exporters.
type RoutingConfig struct {
	// Rules is a list of routing rules that map resource attributes to endpoint properties.
	Rules []RoutingRule `mapstructure:"rules"`
}

// Config defines configuration for exporter_creator.
type Config struct {
	exporterTemplates map[string]exporterTemplate
	// WatchObservers are the extensions to listen to endpoints from.
	WatchObservers []component.ID `mapstructure:"watch_observers"`
	// Routing defines how telemetry is routed to dynamically created exporters.
	Routing RoutingConfig `mapstructure:"routing"`
	// DefaultExporters are static exporters to route unmatched telemetry to.
	DefaultExporters []component.ID `mapstructure:"default_exporters"`
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	// Initialize the map if nil
	if cfg.exporterTemplates == nil {
		cfg.exporterTemplates = make(map[string]exporterTemplate)
	}

	// Validate routing rules
	for i, rule := range cfg.Routing.Rules {
		if rule.ResourceAttribute == "" {
			return fmt.Errorf("routing rule %d: resource_attribute is required", i)
		}
		if rule.EndpointProperty == "" {
			return fmt.Errorf("routing rule %d: endpoint_property is required", i)
		}
	}

	exportersCfg, err := componentParser.Sub(exportersConfigKey)
	if err != nil {
		// No exporters configured is valid
		return nil
	}

	for subexporterKey := range exportersCfg.ToStringMap() {
		subexporterSection, err := exportersCfg.Sub(subexporterKey)
		if err != nil {
			return fmt.Errorf("unable to extract subexporter key %v: %w", subexporterKey, err)
		}
		cfgSection := cast.ToStringMap(subexporterSection.Get(configKey))
		subexporter, err := newExporterTemplate(subexporterKey, cfgSection)
		if err != nil {
			return err
		}

		// Unmarshals exporter_creator configuration like rule.
		if err = subexporterSection.Unmarshal(&subexporter, confmap.WithIgnoreUnused()); err != nil {
			return fmt.Errorf("failed to deserialize sub-exporter %q: %w", subexporterKey, err)
		}

		subexporter.rule, err = newRule(subexporter.Rule)
		if err != nil {
			return fmt.Errorf("subexporter %q rule is invalid: %w", subexporterKey, err)
		}

		for k, v := range subexporter.ResourceAttributes {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("unsupported `resource_attributes` %q value %v in %s", k, v, subexporterKey)
			}
		}

		// If Signals was configured, use it; otherwise use the default (all enabled)
		if subexporter.Signals != (exporterSignals{}) {
			subexporter.signals = subexporter.Signals
		}

		cfg.exporterTemplates[subexporterKey] = subexporter
	}

	return nil
}
