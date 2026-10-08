// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"
	"maps"
	"slices"
	"strings"

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

// NewFactory returns a new factory for the partitioning processor.
func NewFactory() processor.Factory {
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
	return &Config{}
}

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
	c := cfg.(*Config)
	keyNames, expressions := sortedPartitionKeys(c.Keys)
	p, err := newLogsPartitioner(expressions, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	// processorhelper only supports one outgoing batch, so the partitioner runs
	// as its next consumer; the helper still records the processor telemetry.
	pp := &partitioningProcessor{
		nextLogs:        next,
		logsPartitioner: p,
		keyNames:        keyNames,
	}
	return processorhelper.NewLogs(ctx, set, cfg, pp,
		func(_ context.Context, ld plog.Logs) (plog.Logs, error) { return ld, nil },
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	c := cfg.(*Config)
	keyNames, expressions := sortedPartitionKeys(c.Keys)
	p, err := newTracesPartitioner(expressions, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	pp := &partitioningProcessor{
		nextTraces:        next,
		tracesPartitioner: p,
		keyNames:          keyNames,
	}
	return processorhelper.NewTraces(ctx, set, cfg, pp,
		func(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) { return td, nil },
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}

// NOTE: the processors below are no-op passthroughs that forward telemetry
// unchanged. Partitioning for these signals will follow in a subsequent PR.

func createMetricsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	return processorhelper.NewMetrics(ctx, set, cfg, next,
		func(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) { return md, nil },
	)
}

func createProfilesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next xconsumer.Profiles) (xprocessor.Profiles, error) {
	return xprocessorhelper.NewProfiles(ctx, set, cfg, next,
		func(_ context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) { return pd, nil },
	)
}

func sortedPartitionKeys(keys map[string]string) ([]string, []string) {
	names := slices.Sorted(maps.Keys(keys))
	expressions := make([]string, len(names))
	for i, name := range names {
		expressions[i] = keys[name]
		// client.Metadata keys are case-insensitive and stored lower-cased.
		names[i] = strings.ToLower(name)
	}
	return names, expressions
}
