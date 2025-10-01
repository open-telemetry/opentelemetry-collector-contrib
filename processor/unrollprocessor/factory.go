// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unrollprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// componentType is the value of the "type" key in configuration.

// NewFactory returns a new factory for the Transform processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)

	proc, err := newUnrollProcessor(oCfg)
	if err != nil {
		return nil, fmt.Errorf("invalid config for \"unroll\" processor %w", err)
	}
	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.ProcessLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}
