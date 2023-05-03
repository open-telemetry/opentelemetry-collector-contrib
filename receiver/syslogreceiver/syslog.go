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

package syslogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
)

const (
	typeStr   = "syslog"
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for syslog receiver
func NewFactory() receiver.Factory {
	return adapter.NewFactory(ReceiverType{}, stability)
}

// ReceiverType implements adapter.LogReceiverType
// to create a syslog receiver
type ReceiverType struct{}

// Type is the receiver type
func (f ReceiverType) Type() component.Type {
	return typeStr
}

// CreateDefaultConfig creates a config with type and version
func (f ReceiverType) CreateDefaultConfig() component.Config {
	return &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: *syslog.NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f ReceiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*SysLogConfig).BaseConfig
}

// SysLogConfig defines configuration for the syslog receiver
type SysLogConfig struct {
	InputConfig        syslog.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`
}

// InputConfig unmarshals the input operator
func (f ReceiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*SysLogConfig).InputConfig)
}

func (cfg *SysLogConfig) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if componentParser.IsSet("tcp") {
		cfg.InputConfig.TCP = &tcp.NewConfig().BaseConfig
	} else if componentParser.IsSet("udp") {
		cfg.InputConfig.UDP = &udp.NewConfig().BaseConfig
	}

	return componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
}
