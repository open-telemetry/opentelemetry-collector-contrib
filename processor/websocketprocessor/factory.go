// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, metadata.TracesStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createMetricsProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, consumer consumer.Metrics) (processor.Metrics, error) {
	rCfg := cfg.(*Config)
	p, err := newProcessor(params, rCfg)
	if err != nil {
		return nil, err
	}
	p.metricsSink = consumer
	return p, nil
}

func createLogsProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, consumer consumer.Logs) (processor.Logs, error) {
	rCfg := cfg.(*Config)
	p, err := newProcessor(params, rCfg)
	if err != nil {
		return nil, err
	}
	p.logsSink = consumer
	return p, nil
}

func createTraceProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, consumer consumer.Traces) (processor.Traces, error) {
	rCfg := cfg.(*Config)
	p, err := newProcessor(params, rCfg)
	if err != nil {
		return nil, err
	}
	p.tracesSink = consumer
	return p, nil
}
