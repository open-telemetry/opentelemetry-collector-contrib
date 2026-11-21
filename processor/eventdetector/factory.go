// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventdetector // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/eventdetector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/eventdetector/internal/metadata"
)

// NewFactory creates the eventdetector processor factory. The processor can
// be wired into either (or both) the logs and metrics pipelines; each
// pipeline instance shares windows through the configured signalstate
// extension, which is where cross-signal coherence lives.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	core, err := newCore(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	return &detectorLogs{core: core, next: next}, nil
}

func createMetricsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	core, err := newCore(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	return &detectorMetrics{core: core, next: next}, nil
}
