// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/hotreloadprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	config := &Config{
		Region:          "us-east-1",
		RefreshInterval: 60 * time.Second,
		ShutdownDelay:   10 * time.Second,
	}
	return config
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	hp, err := newHotReloadLogsProcessor(ctx, set, cfg.(*Config), nextConsumer)
	if err != nil {
		return nil, err
	}
	return hp, nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	hp, err := newHotReloadMetricsProcessor(ctx, set, cfg.(*Config), nextConsumer)
	if err != nil {
		return nil, err
	}
	return hp, nil
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	hp, err := newHotReloadTracesProcessor(ctx, set, cfg.(*Config), nextConsumer)
	if err != nil {
		return nil, err
	}
	return hp, nil
}
