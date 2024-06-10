// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"sync"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

var onceMetrics sync.Once

// The default precision is 4 hex digits, slightly more the original
// component logic's 14-bits of precision.
const defaultPrecision = 4

// NewFactory returns a new factory for the Probabilistic sampler processor.
func NewFactory() processor.Factory {
	onceMetrics.Do(func() {
		// TODO: Handle this err
		_ = view.Register(samplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		AttributeSource:   defaultAttributeSource,
		FailClosed:        true,
		Mode:              modeUnset,
		SamplingPrecision: defaultPrecision,
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	return newTracesProcessor(ctx, set, cfg.(*Config), nextConsumer)
}

// createLogsProcessor creates a log processor based on this config.
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newLogsProcessor(ctx, set, nextConsumer, cfg.(*Config))
}
