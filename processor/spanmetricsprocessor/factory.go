// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor"

import (
	"context"
	"time"

	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" key in configuration.
	typeStr = "spanmetrics"
	// The stability level of the processor.
	stability = component.StabilityLevelDeprecated
)

// NewFactory creates a factory for the spanmetrics processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality: "AGGREGATION_TEMPORALITY_CUMULATIVE",
		DimensionsCacheSize:    defaultDimensionsCacheSize,
		skipSanitizeLabel:      dropSanitizationGate.IsEnabled(),
		MetricsFlushInterval:   15 * time.Second,
	}
}

func createTracesProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	p, err := newProcessor(params.Logger, cfg, metricsTicker(ctx, cfg))
	if err != nil {
		return nil, err
	}
	p.tracesConsumer = nextConsumer
	return p, nil
}

func metricsTicker(ctx context.Context, cfg component.Config) *clock.Ticker {
	return clock.FromContext(ctx).NewTicker(cfg.(*Config).MetricsFlushInterval)
}
