// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator

import (
	"fmt"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configparser"

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
	id config.ComponentID
	// config is the map configured by the user in the config file. It is the contents of the map from
	// the "config" section. The keys and values are arbitrarily configured by the user.
	config userConfigMap
}

// userConfigMap is an arbitrary map of string keys to arbitrary values as specified by the user
type userConfigMap map[string]interface{}

// receiverTemplate is the configuration of a single subreceiver.
type receiverTemplate struct {
	receiverConfig

	// Rule is the discovery rule that when matched will create a receiver instance
	// based on receiverTemplate.
	Rule string `mapstructure:"rule"`
	rule rule
}

// resourceAttributes holds a map of default resource attributes for each Endpoint type.
type resourceAttributes map[observer.EndpointType]map[string]string

// newReceiverTemplate creates a receiverTemplate instance from the full name of a subreceiver
// and its arbitrary config map values.
func newReceiverTemplate(name string, cfg userConfigMap) (receiverTemplate, error) {
	id, err := config.NewIDFromString(name)
	if err != nil {
		return receiverTemplate{}, err
	}

	return receiverTemplate{
		receiverConfig: receiverConfig{
			id:     id,
			config: cfg,
		},
	}, nil
}

var _ config.Unmarshallable = (*Config)(nil)

// Config defines configuration for receiver_creator.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	receiverTemplates       map[string]receiverTemplate
	// WatchObservers are the extensions to listen to endpoints from.
	WatchObservers []config.Type `mapstructure:"watch_observers"`
	// ResourceAttributes is a map of default resource attributes to add to each resource
	// object received by this receiver from dynamically created receivers.
	ResourceAttributes resourceAttributes `mapstructure:"resource_attributes"`
}

func (cfg *Config) Unmarshal(componentParser *configparser.Parser) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg); err != nil {
		return err
	}

	receiversCfg, err := componentParser.Sub(receiversConfigKey)
	if err != nil {
		return fmt.Errorf("unable to extract key %v: %v", receiversConfigKey, err)
	}

	for subreceiverKey := range receiversCfg.ToStringMap() {
		subreceiverSection, err := receiversCfg.Sub(subreceiverKey)
		if err != nil {
			return fmt.Errorf("unable to extract subreceiver key %v: %v", subreceiverKey, err)
		}
		cfgSection := cast.ToStringMap(subreceiverSection.Get(configKey))
		subreceiver, err := newReceiverTemplate(subreceiverKey, cfgSection)
		if err != nil {
			return err
		}

		// Unmarshals receiver_creator configuration like rule.
		if err = subreceiverSection.Unmarshal(&subreceiver); err != nil {
			return fmt.Errorf("failed to deserialize sub-receiver %q: %s", subreceiverKey, err)
		}

		subreceiver.rule, err = newRule(subreceiver.Rule)
		if err != nil {
			return fmt.Errorf("subreceiver %q rule is invalid: %v", subreceiverKey, err)
		}

		cfg.receiverTemplates[subreceiverKey] = subreceiver
	}

	return nil
}
