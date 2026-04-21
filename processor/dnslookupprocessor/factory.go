// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
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

func createMetricsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	return processorhelper.NewMetrics(ctx, set, cfg, nextConsumer, newDNSLookupProcessor().processMetrics, processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	return processorhelper.NewTraces(ctx, set, cfg, nextConsumer, newDNSLookupProcessor().processTraces, processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	return processorhelper.NewLogs(ctx, set, cfg, nextConsumer, newDNSLookupProcessor().processLogs, processorhelper.WithCapabilities(processorCapabilities))
}
