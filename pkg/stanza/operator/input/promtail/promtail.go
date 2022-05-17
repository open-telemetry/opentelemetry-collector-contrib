package promtail

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/clients/pkg/promtail/positions"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/grafana/loki/clients/pkg/promtail/targets"
	"github.com/grafana/loki/clients/pkg/promtail/targets/file"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("promtail_input", func() operator.Builder { return NewPromtailInputConfig("") })
}

func NewPromtailInputConfig(operatorID string) *PromtailInputConfig {
	return &PromtailInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "promtail_input"),
	}
}

// PromtailInputConfig is the configuration of a journald input operator
type PromtailInputConfig struct {
	helper.InputConfig `mapstructure:",squash" yaml:",inline"`

	PromtailConfig *PromtailConfig `mapstructure:"config" yaml:"config"`
}

type PromtailConfig struct {
	PositionsConfig positions.Config      `mapstructure:"positions,omitempty" json:"positions,omitempty" yaml:"positions,omitempty"`
	ScrapeConfig    []scrapeconfig.Config `mapstructure:"scrape_configs,omitempty" json:"scrape_configs,omitempty" yaml:"scrape_configs,omitempty"`
	TargetConfig    file.Config           `mapstructure:"target_config,omitempty" json:"target_config,omitempty" yaml:"target_config,omitempty"`
}

// Build will build a promtail input operator from the supplied configuration
func (c PromtailInputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if len(c.PromtailConfig.ScrapeConfig) == 0 {
		return nil, fmt.Errorf("required argument `scrape_configs` is empty")
	}
	if c.PromtailConfig.PositionsConfig.PositionsFile == "" {
		c.PromtailConfig.PositionsConfig.PositionsFile = "/var/log/positions.yaml"
	}
	if c.PromtailConfig.PositionsConfig.SyncPeriod == 0 {
		c.PromtailConfig.PositionsConfig.SyncPeriod = 10 * time.Second
	}
	if c.PromtailConfig.TargetConfig.SyncPeriod == 0 {
		return nil, fmt.Errorf("required argument `target_configs.sync_period` is empty")
	}

	entries := make(chan api.Entry)

	return &PromtailInput{
		InputOperator: inputOperator,
		config:        c.PromtailConfig,
		app: &app{
			client:  api.NewEntryHandler(entries, func() { close(entries) }),
			entries: entries,
			// logger:  helper.NewZapToGokitLogAdapter(logger.Desugar()),
			logger: log.NewNopLogger(),
			reg:    prometheus.DefaultRegisterer,
		},
	}, nil
}

type PromtailInput struct {
	helper.InputOperator
	config *PromtailConfig
	app    *app
	cancel context.CancelFunc
}

type app struct {
	manager *targets.TargetManagers
	client  api.EntryHandler
	entries chan api.Entry
	logger  log.Logger
	reg     prometheus.Registerer
}

func (a *app) Shutdown() {
	if a.manager != nil {
		a.manager.Stop()
	}
	a.client.Stop()
}

func (operator *PromtailInput) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	operator.cancel = cancel

	manager, err := targets.NewTargetManagers(
		operator.app,
		operator.app.reg,
		operator.app.logger,
		operator.config.PositionsConfig,
		operator.app.client,
		operator.config.ScrapeConfig,
		&operator.config.TargetConfig,
	)

	if err != nil {
		return err
	}
	operator.app.manager = manager

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case inputEntry := <-operator.app.entries:
				entry, err := operator.parsePromtailEntry(inputEntry)
				if err != nil {
					operator.Warnw("Failed to parse promtail entry", zap.Error(err))
					continue
				}
				operator.Write(ctx, entry)
			}
		}
	}()
	return nil
}

func (operator *PromtailInput) Stop() error {
	operator.cancel()
	operator.app.Shutdown()
	return nil
}

func (operator *PromtailInput) parsePromtailEntry(inputEntry api.Entry) (*entry.Entry, error) {
	outputEntry, err := operator.NewEntry(inputEntry.Entry.Line)
	if err != nil {
		return nil, err
	}
	outputEntry.Timestamp = inputEntry.Entry.Timestamp

	for key, val := range inputEntry.Labels {
		valStr := string(val)
		keyStr := string(key)
		switch key {
		case "filename":
			outputEntry.AddAttribute("log.file.path", valStr)
			outputEntry.AddAttribute("log.file.name", path.Base(valStr))
		default:
			outputEntry.AddAttribute(keyStr, valStr)
		}
	}
	return outputEntry, nil
}
