// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerprofiles"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processorhelper/processorhelperprofiles"
	"go.opentelemetry.io/collector/processor/processorprofiles"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Resource processor.
func NewFactory() processor.Factory {
	return processorprofiles.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processorprofiles.WithTraces(createTracesProcessor, metadata.TracesStability),
		processorprofiles.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processorprofiles.WithLogs(createLogsProcessor, metadata.LogsStability),
		processorprofiles.WithProfiles(createProfilesProcessor, metadata.ProfilesStability),
	)
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	attrProc, err := attraction.NewAttrProc(&attraction.Settings{Actions: cfg.(*Config).AttributesActions})
	if err != nil {
		return nil, err
	}
	proc := &resourceProcessor{logger: set.Logger, attrProc: attrProc}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	attrProc, err := attraction.NewAttrProc(&attraction.Settings{Actions: cfg.(*Config).AttributesActions})
	if err != nil {
		return nil, err
	}
	proc := &resourceProcessor{logger: set.Logger, attrProc: attrProc}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	attrProc, err := attraction.NewAttrProc(&attraction.Settings{Actions: cfg.(*Config).AttributesActions})
	if err != nil {
		return nil, err
	}
	proc := &resourceProcessor{logger: set.Logger, attrProc: attrProc}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumerprofiles.Profiles,
) (processorprofiles.Profiles, error) {
	attrProc, err := attraction.NewAttrProc(&attraction.Settings{Actions: cfg.(*Config).AttributesActions})
	if err != nil {
		return nil, err
	}
	proc := resourceProcessor{logger: set.Logger, attrProc: attrProc}
	return processorhelperprofiles.NewProfiles(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processProfiles,
		processorhelperprofiles.WithCapabilities(processorCapabilities))
}
