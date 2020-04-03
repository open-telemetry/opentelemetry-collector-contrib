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
	"github.com/open-telemetry/opentelemetry-collector/component"
	otelconfig "github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

type factoryLookup interface {
	// GetFactory of the specified kind. Returns the factory for a component type.
	GetFactory(kind component.Kind, componentType string) component.Factory
}

// userConfigMap is an arbitrary map of string keys to arbitrary values as specified by the user
type userConfigMap map[string]interface{}

// subreceiverConfig is the configuration of a single subreceiver configured inside
// receiver_creator.
type subreceiverConfig struct {
	// Rule is the discovery rule that when matched will create a receiver instance
	// based on subreceiverConfig.
	Rule string `mapstructure:"rule"`

	// receiverType is set based on the configured receiver name.
	receiverType string
	// config is the map configured by the user in the config file. It is the contents of the map from
	// the "config" section. The keys and values are arbitrarily configured by the user.
	config userConfigMap
	// fullName is the full subreceiver name (ie <receiver type>/<id>).
	fullName string
}

// newSubreceiverConfig creates a subreceiverConfig instance from the full name of a subreceiver
// and its arbitrary config map values.
func newSubreceiverConfig(name string, config userConfigMap) (*subreceiverConfig, error) {
	typeStr, fullName, err := otelconfig.DecodeTypeAndName(name)
	if err != nil {
		return nil, err
	}

	return &subreceiverConfig{
		receiverType: typeStr,
		fullName:     fullName,
		config:       config,
	}, nil
}

// Config defines configuration for receiver_creator.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	subreceiverConfigs            map[string]*subreceiverConfig
}
