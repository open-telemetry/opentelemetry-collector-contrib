// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" Tail Sampling in configuration.
	typeStr = "tail_sampling"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var onceMetrics sync.Once

// NewFactory returns a new factory for the Tail Sampling processor.
func NewFactory() processor.Factory {
	onceMetrics.Do(func() {
		// TODO: this is hardcoding the metrics level and skips error handling
		_ = view.Register(SamplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		DecisionWait: 30 * time.Second,
		NumTraces:    50000,
	}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	tCfg := cfg.(*Config)
	return newTracesProcessor(ctx, params.TelemetrySettings, nextConsumer, *tCfg)
}
