// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Factory for prometheusexec
const (
	defaultCollectionInterval = 60 * time.Second
	defaultTimeoutInterval    = 10 * time.Second
)

var once sync.Once

// NewFactory creates a factory for the prometheusexec receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("prometheus_exec receiver is deprecated and will be removed in future versions.")
	})
}

// createDefaultConfig returns a default config
func createDefaultConfig() component.Config {
	return &Config{
		ScrapeInterval: defaultCollectionInterval,
		ScrapeTimeout:  defaultTimeoutInterval,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Env: []subprocessmanager.EnvConfig{},
		},
	}
}

// createMetricsReceiver creates a metrics receiver based on provided Config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	logDeprecation(params.Logger)
	rCfg := cfg.(*Config)
	return newPromExecReceiver(params, rCfg, nextConsumer), nil
}
