// Copyright The OpenTelemetry Authors
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

package stanzareceiver

import (
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"gopkg.in/yaml.v2"
)

// StanzaConfig defines a base configuration for stanza-based receivers
type StanzaConfig struct {
	Operators OperatorConfig
	// OffsetsFile                   string         `mapstructure:"offsets_file"`
	// PluginDir                     string         `mapstructure:"plugin_dir"`
}

// OperatorConfig is an alias that allows for unmarshaling outside of mapstructure
// Stanza operators should will be migrated to mapstructure for greater compatibility
// but this allows a temporary solution
type OperatorConfig []map[string]interface{}

func DecodeOperators(cfg OperatorConfig) (pipeline.Config, error) {
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var pipelineCfg pipeline.Config
	if err := yaml.Unmarshal(yamlBytes, &pipelineCfg); err != nil {
		return nil, err
	}
	return pipelineCfg, nil
}
