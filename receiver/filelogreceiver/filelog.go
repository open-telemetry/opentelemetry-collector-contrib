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

package filelogreceiver

import (
	// Register input operator for filelog
	_ "github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/file"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stanzareceiver"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"gopkg.in/yaml.v2"
)

const (
	typeStr = "filelog"
	verStr  = "0.14.0"
)

// NewFactory creates a factory for filelog receiver
func NewFactory() component.ReceiverFactory {
	return stanzareceiver.NewFactory(FileLogReceiverType{})
}

type FileLogReceiverType struct {
	stanzareceiver.LogReceiverType
}

func (f FileLogReceiverType) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

func (f FileLogReceiverType) Version() string {
	return verStr
}

func (f FileLogReceiverType) CreateDefaultConfig() configmodels.Receiver {
	return &FileLogConfig{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
	}
}

func (f FileLogReceiverType) Decode(cfg configmodels.Receiver) (pipeline.Config, error) {
	logConfig := cfg.(*FileLogConfig)

	yamlBytes, err := yaml.Marshal(logConfig.Operators)
	if err != nil {
		return nil, err
	}

	var pipelineCfg pipeline.Config
	if err := yaml.Unmarshal(yamlBytes, &pipelineCfg); err != nil {
		return nil, err
	}
	return pipelineCfg, nil
}

// FileLogConfig defines configuration for the stanza receiver
type FileLogConfig struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`
	Operators                     OperatorConfig `mapstructure:"operators"`
	// OffsetsFile                   string         `mapstructure:"offsets_file"`
	// PluginDir                     string         `mapstructure:"plugin_dir"`
}

// OperatorConfig is an alias that allows for unmarshaling outside of mapstructure
// Stanza operators should be migrated to mapstructure for greater compatibility
// but this allows a temporary solution
type OperatorConfig []map[string]interface{}
