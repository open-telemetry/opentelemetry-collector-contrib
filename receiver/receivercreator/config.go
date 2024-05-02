// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	// receiversConfigKey is the config key name used to specify the subreceivers.
	receiversConfigKey = "receivers"
	// endpointConfigKey is the key name mapping to ReceiverSettings.Endpoint.
	endpointConfigKey = "endpoint"
	// configKey is the key name in a subreceiver.
	configKey = "config"
)

// receiverConfig describes a receiver instance with a default config.
type receiverConfig struct {
	// id is the id of the subreceiver (ie <receiver type>/<id>).
	id component.ID
	// config is the map configured by the user in the config file. It is the contents of the map from
	// the "config" section. The keys and values are arbitrarily configured by the user.
	config     userConfigMap
	endpointID observer.EndpointID
}

// userConfigMap is an arbitrary map of string keys to arbitrary values as specified by the user
type userConfigMap map[string]any

// receiverTemplate is the configuration of a single subreceiver.
type receiverTemplate struct {
	receiverConfig

	// Rule is the discovery rule that when matched will create a receiver instance
	// based on receiverTemplate.
	Rule string `mapstructure:"rule"`
	// ResourceAttributes is a map of resource attributes to add to just this receiver's resource metrics.
	// It can contain expr expressions for endpoint env value expansion
	ResourceAttributes map[string]any `mapstructure:"resource_attributes"`
	rule               rule
}

// resourceAttributes holds a map of default resource attributes for each Endpoint type.
type resourceAttributes map[observer.EndpointType]map[string]string

// newReceiverTemplate creates a receiverTemplate instance from the full name of a subreceiver
// and its arbitrary config map values.
func newReceiverTemplate(name string, cfg userConfigMap) (receiverTemplate, error) {
	id := component.ID{}
	if err := id.UnmarshalText([]byte(name)); err != nil {
		return receiverTemplate{}, err
	}

	return receiverTemplate{
		receiverConfig: receiverConfig{
			id:         id,
			config:     cfg,
			endpointID: observer.EndpointID("endpoint.id"),
		},
	}, nil
}

var _ confmap.Unmarshaler = (*Config)(nil)

// Config defines configuration for receiver_creator.
type Config struct {
	receiverTemplates map[string]receiverTemplate
	// WatchObservers are the extensions to listen to endpoints from.
	WatchObservers []component.ID `mapstructure:"watch_observers"`
	// ResourceAttributes is a map of default resource attributes to add to each resource
	// object received by this receiver from dynamically created receivers.
	ResourceAttributes resourceAttributes `mapstructure:"resource_attributes"`
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	for endpointType := range cfg.ResourceAttributes {
		switch endpointType {
		case observer.ContainerType, observer.K8sServiceType, observer.HostPortType, observer.K8sNodeType, observer.PodType, observer.PortType:
		default:
			return fmt.Errorf("resource attributes for unsupported endpoint type %q", endpointType)
		}
	}

	receiversCfg, err := componentParser.Sub(receiversConfigKey)
	if err != nil {
		return fmt.Errorf("unable to extract key %v: %w", receiversConfigKey, err)
	}

	for subreceiverKey := range receiversCfg.ToStringMap() {
		subreceiverSection, err := receiversCfg.Sub(subreceiverKey)
		if err != nil {
			return fmt.Errorf("unable to extract subreceiver key %v: %w", subreceiverKey, err)
		}
		cfgSection := cast.ToStringMap(subreceiverSection.Get(configKey))
		subreceiver, err := newReceiverTemplate(subreceiverKey, cfgSection)
		if err != nil {
			return err
		}

		// Unmarshals receiver_creator configuration like rule.
		if err = subreceiverSection.Unmarshal(&subreceiver, confmap.WithIgnoreUnused()); err != nil {
			return fmt.Errorf("failed to deserialize sub-receiver %q: %w", subreceiverKey, err)
		}

		subreceiver.rule, err = newRule(subreceiver.Rule)
		if err != nil {
			return fmt.Errorf("subreceiver %q rule is invalid: %w", subreceiverKey, err)
		}

		for k, v := range subreceiver.ResourceAttributes {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("unsupported `resource_attributes` %q value %v in %s", k, v, subreceiverKey)
			}
		}

		cfg.receiverTemplates[subreceiverKey] = subreceiver
	}

	return nil
}
