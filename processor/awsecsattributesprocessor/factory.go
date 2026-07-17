// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"
	"fmt"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
)

// NewFactory creates a factory for the awsecsattributes processor.
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
	return &Config{
		CacheTTL: 300, // 5 minutes
		// Attributes is left empty so that, by default, all available ECS
		// metadata attributes are collected.
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
}

// NOTE: this is the initial skeleton donation PR. The processors below are
// no-op passthroughs that forward telemetry unchanged. The ECS metadata
// enrichment logic is added in a follow-up PR.

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

func createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	if _, ok := cfg.(*Config); !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return xprocessorhelper.NewProfiles(
		ctx, set, cfg, nextConsumer,
		func(_ context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) { return pd, nil },
	)
}
