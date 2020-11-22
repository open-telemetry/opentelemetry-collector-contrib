// Copyright 2019, OpenTelemetry Authors
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
	"github.com/observiq/stanza/pipeline"
	"go.opentelemetry.io/collector/config/configmodels"
	"gopkg.in/yaml.v2"
)

// Config defines configuration for the stanza receiver
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	OffsetsFile                   string         `mapstructure:"offsets_file"`
	PluginDir                     string         `mapstructure:"plugin_dir"`
	Operators                     OperatorConfig `mapstructure:"operators"`
}

type OperatorConfig []map[string]interface{}

func (r OperatorConfig) IntoPipelineConfig() (pipeline.Config, error) {
	yamlBytes, err := yaml.Marshal(r)
	if err != nil {
		return nil, err
	}

	var cfg pipeline.Config
	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
