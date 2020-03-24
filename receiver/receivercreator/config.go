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
	otelconfig "github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

type subreceiverConfig struct {
	// Rule is the discovery rule that when matched will create an subreceiverConfig
	// of this receiver
	Rule string `mapstructure:"rule"`

	// receiverType is set based on the configured name
	receiverType string
	// config is the subreceiver config definition
	config map[string]interface{}
	// fullName is the full subreceiver name (ie <receiver type>/<id>)
	fullName string
}

func newSubreceiverConfig(name string, config map[string]interface{}) (*subreceiverConfig, error) {
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
