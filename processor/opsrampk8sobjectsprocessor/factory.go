// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampk8sobjectsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sobjectsprocessor"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampk8sobjectsprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Attributes processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		newOpsrampK8sObjectsProcessor(set.Logger).processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}
