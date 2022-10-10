package promtailreceiver

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
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
func (f ReceiverType) Type() config.Type {
	return typeStr
}

// CreateDefaultConfig creates a config with type and version
func (f ReceiverType) CreateDefaultConfig() config.Receiver {
	return &PromtailConfig{
		BaseConfig: adapter.BaseConfig{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
			Operators:        []operator.Config{},
		},
		InputConfig: *NewConfig(),
	}
}

// BaseConfig gets the base config from config, for now
func (f ReceiverType) BaseConfig(cfg config.Receiver) adapter.BaseConfig {
	return cfg.(*PromtailConfig).BaseConfig
}

// InputConfig unmarshals the input operator
func (f ReceiverType) InputConfig(cfg config.Receiver) operator.Config {
	return operator.NewConfig(&cfg.(*PromtailConfig).InputConfig)
}
