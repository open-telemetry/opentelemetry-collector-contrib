// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promtailreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const (
	typeStr   = "promtail"
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for promtail receiver
func NewFactory() component.ReceiverFactory {
	return adapter.NewFactory(ReceiverType{}, stability)
}

// ReceiverType implements stanza.LogReceiverType
// to create a promtail receiver
type ReceiverType struct{}

// Type is the receiver type
func (f ReceiverType) Type() component.Type {
	return typeStr
}

// CreateDefaultConfig creates a config with type and version
func (f ReceiverType) CreateDefaultConfig() component.ReceiverConfig {
	return &PromtailConfig{
		BaseConfig: adapter.BaseConfig{
			ReceiverSettings: config.NewReceiverSettings(component.NewID(typeStr)),
			Operators:        []operator.Config{},
		},
		InputConfig: *NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f ReceiverType) BaseConfig(cfg component.ReceiverConfig) adapter.BaseConfig {
	return cfg.(*PromtailConfig).BaseConfig
}

// InputConfig unmarshals the input operator
func (f ReceiverType) InputConfig(cfg component.ReceiverConfig) operator.Config {
	return operator.NewConfig(&cfg.(*PromtailConfig).InputConfig)
}
