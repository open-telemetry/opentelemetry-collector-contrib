// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/gnetexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
)

const (
	defaultDialTimeout        = 5 * time.Second
	defaultWriteTimeout       = 10 * time.Second
	defaultReconnectDelay     = 1 * time.Second
	defaultMaxReconnectDelay  = 30 * time.Second
)

// NewFactory creates a factory for the gnet exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType("gnet"),
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelDevelopment),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelDevelopment),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:          "localhost:4320",
		DialTimeout:       defaultDialTimeout,
		WriteTimeout:      defaultWriteTimeout,
		ReconnectDelay:    defaultReconnectDelay,
		MaxReconnectDelay: defaultMaxReconnectDelay,
	}
}

func createTracesExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	e, err := newExporter(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return exporter.NewTraces(
		e.consumeTraces,
		exporter.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporter.WithStart(e.Start),
		exporter.WithShutdown(e.Shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	e, err := newExporter(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return exporter.NewMetrics(
		e.consumeMetrics,
		exporter.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporter.WithStart(e.Start),
		exporter.WithShutdown(e.Shutdown),
	)
}

func createLogsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	e, err := newExporter(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	return exporter.NewLogs(
		e.consumeLogs,
		exporter.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporter.WithStart(e.Start),
		exporter.WithShutdown(e.Shutdown),
	)
}
