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
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const (
	typeStr   = "promtail"
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for promtail receiver
func NewFactory() receiver.Factory {
	return adapter.NewFactory(receiverType{}, stability)
}

// receiverType implements stanza.LogReceiverType
// to create a promtail receiver
type receiverType struct{}

// Type is the receiver type
func (f receiverType) Type() component.Type {
	return typeStr
}

// CreateDefaultConfig creates a config with type and version
func (f receiverType) CreateDefaultConfig() component.Config {
	return &PromtailConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: *newConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f receiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*PromtailConfig).BaseConfig
}

// InputConfig unmarshals the input operator
func (f receiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*PromtailConfig).InputConfig)
}
