// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		GroupByKeys: []string{},
	}
}

func createGroupByAttrsProcessor(set processor.Settings, attributes []string) (*groupByAttrsProcessor, error) {
	var nonEmptyAttributes []string
	presentAttributes := make(map[string]struct{})

	for _, str := range attributes {
		if str != "" {
			_, isPresent := presentAttributes[str]
			if isPresent {
				set.Logger.Warn("A grouping key is already present", zap.String("key", str))
			} else {
				nonEmptyAttributes = append(nonEmptyAttributes, str)
				presentAttributes[str] = struct{}{}
			}
		}
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &groupByAttrsProcessor{logger: set.Logger, groupByKeys: nonEmptyAttributes, telemetryBuilder: telemetryBuilder}, nil
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	gap, err := createGroupByAttrsProcessor(set, oCfg.GroupByKeys)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// createLogsProcessor creates a logs processor based on this config.
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	gap, err := createGroupByAttrsProcessor(set, oCfg.GroupByKeys)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// createMetricsProcessor creates a metrics processor based on this config.
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	gap, err := createGroupByAttrsProcessor(set, oCfg.GroupByKeys)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities))
}
