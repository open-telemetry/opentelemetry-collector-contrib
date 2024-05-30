// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory creates a new processor factory with default configuration,
// and registers the processors for metrics, traces, and logs.
func NewFactory() processor.Factory {
	return processor.NewFactory(metadata.Type, createDefaultConfig, processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability), processor.WithLogs(createLogsProcessor, metadata.LogsStability), processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	return processorhelper.NewMetricsProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor().processMetrics, processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	return processorhelper.NewTracesProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor().processTraces, processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(ctx context.Context, set processor.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	return processorhelper.NewLogsProcessor(ctx, set, cfg, nextConsumer, newGeoIPProcessor().processLogs, processorhelper.WithCapabilities(processorCapabilities))
}
