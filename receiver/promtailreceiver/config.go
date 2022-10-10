package promtailreceiver

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver/internal"
	"go.opentelemetry.io/collector/confmap"

	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// PromtailConfig defines configuration for the promtail receiver
type PromtailConfig struct {
	InputConfig        Config `mapstructure:",squash" yaml:",inline"`
	adapter.BaseConfig `mapstructure:",squash" yaml:",inline"`
}

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
	}
}

type PromtailInputConfig struct {
	PositionsConfig positions.Config      `mapstructure:"positions,omitempty" yaml:"positions,omitempty"`
	ScrapeConfig    []scrapeconfig.Config `mapstructure:"scrape_configs,omitempty" yaml:"scrape_configs,omitempty"`
	TargetConfig    file.Config           `mapstructure:"target_config,omitempty" yaml:"target_config,omitempty"`
}

type Config struct {
	helper.InputConfig `mapstructure:",squash" yaml:",inline"`
	// Promtail input config is declared as nested key to allow custom unmarshalling
	// confmap doesn't call custom unmarshal for the top structure
	// see the implementation of unmarshalerHookFunc in https://github.com/open-telemetry/opentelemetry-collector:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/confmap/confmap.go#L274
	Input PromtailInputConfig `mapstructure:"config,omitempty" yaml:"config,omitempty"`
}

// Build will build a promtail input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}
	if len(c.Input.ScrapeConfig) == 0 {
		return nil, fmt.Errorf("required argument `scrape_configs` is empty")
	}
	if c.Input.PositionsConfig.PositionsFile == "" {
		c.Input.PositionsConfig.PositionsFile = "/var/log/positions.yaml"
	}
	if c.Input.PositionsConfig.SyncPeriod == 0 {
		c.Input.PositionsConfig.SyncPeriod = 10 * time.Second
	}
	if c.Input.TargetConfig.SyncPeriod == 0 {
		return nil, fmt.Errorf("required argument `target_configs.sync_period` is empty")
	}

	entries := make(chan api.Entry)

	return &PromtailInput{
		InputOperator: inputOperator,
		config:        &c,
		app: &app{
			client:  api.NewEntryHandler(entries, func() { close(entries) }),
			entries: entries,
			logger:  internal.NewZapToGokitLogAdapter(logger.Desugar()),
			reg:     prometheus.DefaultRegisterer,
		},
	}, nil
}

// Unmarshal a config.Parser into the config struct.
func (c *PromtailInputConfig) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}
	out, err := yaml.Marshal(componentParser.ToStringMap())
	if err != nil {
		return fmt.Errorf("promtail receiver failed to marshal config to yaml: %w", err)
	}

	err = yaml.UnmarshalStrict(out, c)
	if err != nil {
		return fmt.Errorf("promtail receiver failed to unmarshal yaml to promtail config: %w", err)
	}
	return nil
}
