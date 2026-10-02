// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor/internal/metadata"
)

// NewFactory returns a new xprocessor.Factory for the partitioning processor.
func NewFactory() xprocessor.Factory {
	return xprocessor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xprocessor.WithLogs(createLogsProcessor, metadata.LogsStability),
		xprocessor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		xprocessor.WithTraces(createTracesProcessor, metadata.TracesStability),
		xprocessor.WithProfiles(createProfilesProcessor, metadata.ProfilesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{MaxConcurrentPartitions: defaultMaxConcurrentPartitions()}
}

// NOTE: this is the initial skeleton donation PR. The processors below are
// no-op passthroughs that forward telemetry unchanged. The partitioning logic
// will follow in subsequent PRs.

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
	return processorhelper.NewLogs(ctx, set, cfg, next,
		func(_ context.Context, ld plog.Logs) (plog.Logs, error) { return ld, nil },
	)
}

func createMetricsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	return processorhelper.NewMetrics(ctx, set, cfg, next,
		func(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) { return md, nil },
	)
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	return processorhelper.NewTraces(ctx, set, cfg, next,
		func(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) { return td, nil },
	)
}

func createProfilesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next xconsumer.Profiles) (xprocessor.Profiles, error) {
	return xprocessorhelper.NewProfiles(ctx, set, cfg, next,
		func(_ context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) { return pd, nil },
	)
}
