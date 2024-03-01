// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

var onceMetrics sync.Once

var metricStatCountSpansSampledFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"processor.tailsamplingprocessor.metricstatcountspanssampled",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, a new metric stat_count_spans_sampled will be available in the tail sampling processor. Differently from stat_count_traces_sampled, this metric will count the number of spans sampled or not per sampling policy, where the original counts traces."),
)

func isMetricStatCountSpansSampledEnabled() bool {
	return metricStatCountSpansSampledFeatureGate.IsEnabled()
}

// NewFactory returns a new factory for the Tail Sampling processor.
func NewFactory() processor.Factory {
	onceMetrics.Do(func() {
		// TODO: this is hardcoding the metrics level and skips error handling
		_ = view.Register(samplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
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
