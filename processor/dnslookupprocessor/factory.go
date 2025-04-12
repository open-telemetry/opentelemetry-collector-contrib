// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
)

// SourceIPKey semconv does not define a Key for source IP. When semconv defines it, this should be removed.
const SourceIPKey = "source.ip"

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory creates a new processor factory with default configuration,
// and registers the processors for metrics, traces, and logs.
func NewFactory() processor.Factory {
	return processor.NewFactory(metadata.Type, createDefaultConfig, processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability), processor.WithLogs(createLogsProcessor, metadata.LogsStability), processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

// createDefaultConfig returns a default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		Resolve: LookupConfig{
			Enabled:           true,
			Context:           resource,
			Attributes:        []string{string(semconv.SourceAddressKey)},
			ResolvedAttribute: SourceIPKey,
		},
		Reverse: LookupConfig{
			Enabled:           false,
			Context:           resource,
			Attributes:        []string{SourceIPKey},
			ResolvedAttribute: string(semconv.SourceAddressKey),
		},
		HitCacheSize:         1000,
		HitCacheTTL:          60,
		MissCacheSize:        1000,
		MissCacheTTL:         30,
		MaxRetries:           1,
		Timeout:              0.5,
		EnableSystemResolver: true,
	}
}

func createMetricsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	config := cfg.(*Config)
	dp, err := newDNSLookupProcessor(config, set.Logger)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetrics(ctx, set, cfg, nextConsumer, dp.processMetrics, processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	config := cfg.(*Config)
	dp, err := newDNSLookupProcessor(config, set.Logger)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(ctx, set, cfg, nextConsumer, dp.processTraces, processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	config := cfg.(*Config)
	dp, err := newDNSLookupProcessor(config, set.Logger)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(ctx, set, cfg, nextConsumer, dp.processLogs, processorhelper.WithCapabilities(processorCapabilities))
}
