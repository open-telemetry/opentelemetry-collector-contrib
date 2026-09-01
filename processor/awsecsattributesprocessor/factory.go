// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
)

// NewFactory creates a factory for the aws_ecs_attributes processor.
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

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return newLogsProcessor(ctx, set, config, nextConsumer, getEndpoints)
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return newMetricsProcessor(ctx, set, config, nextConsumer, getEndpoints)
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return newTracesProcessor(ctx, set, config, nextConsumer, getEndpoints)
}

func createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	return newProfilesProcessor(ctx, set, config, nextConsumer, getEndpoints)
}
