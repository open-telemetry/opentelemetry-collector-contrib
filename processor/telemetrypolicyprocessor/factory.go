// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrypolicyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/telemetrypolicyprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/telemetrypolicyprocessor/internal/metadata"
)

// NewFactory returns a new factory for the telemetry_policy processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

// NOTE: this is the initial skeleton donation PR. The processors below are
// no-op passthroughs that forward telemetry unchanged. The policy evaluation
// logic will follow in subsequent PRs.

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	if _, ok := cfg.(*Config); !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return processorhelper.NewLogs(
		ctx, set, cfg, nextConsumer,
		func(_ context.Context, ld plog.Logs) (plog.Logs, error) { return ld, nil },
	)
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	if _, ok := cfg.(*Config); !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return processorhelper.NewMetrics(
		ctx, set, cfg, nextConsumer,
		func(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) { return md, nil },
	)
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	if _, ok := cfg.(*Config); !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return processorhelper.NewTraces(
		ctx, set, cfg, nextConsumer,
		func(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) { return td, nil },
	)
}
