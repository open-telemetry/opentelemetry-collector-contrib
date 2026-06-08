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

func createLogsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	if err := config.init(); err != nil {
		return nil, err
	}
	return newLogsProcessor(set.Logger, config, nextConsumer, getEndpoints), nil
}

func createMetricsProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	if err := config.init(); err != nil {
		return nil, err
	}
	return newMetricsProcessor(set.Logger, config, nextConsumer, getEndpoints), nil
}

func createTracesProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	if err := config.init(); err != nil {
		return nil, err
	}
	return newTracesProcessor(set.Logger, config, nextConsumer, getEndpoints), nil
}

func createProfilesProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", metadata.Type.String())
	}
	if err := config.init(); err != nil {
		return nil, err
	}
	return newProfilesProcessor(set.Logger, config, nextConsumer, getEndpoints), nil
}
