package promtailreceiver

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/promtailreceiver/internal"

	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PromtailConfig defines configuration for the promtail receiver
type PromtailConfig struct {
	adapter.BaseConfig `mapstructure:",squash"`
	InputConfig        Config `mapstructure:",squash"`
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

type Config struct {
	helper.InputConfig `mapstructure:",squash"`

	PositionsConfig positions.Config      `mapstructure:"positions,omitempty"`
	ScrapeConfig    []scrapeconfig.Config `mapstructure:"scrape_configs,omitempty"`
	TargetConfig    file.Config           `mapstructure:"target_config,omitempty"`
}

// Build will build a promtail input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}
	if len(c.ScrapeConfig) == 0 {
		return nil, fmt.Errorf("required argument `scrape_configs` is empty")
	}
	if c.PositionsConfig.PositionsFile == "" {
		c.PositionsConfig.PositionsFile = "/var/log/positions.yaml"
	}
	if c.PositionsConfig.SyncPeriod == 0 {
		c.PositionsConfig.SyncPeriod = 10 * time.Second
	}
	if c.TargetConfig.SyncPeriod == 0 {
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
