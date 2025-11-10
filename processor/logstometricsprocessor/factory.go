// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstometricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/metadata"
)

// NewFactory creates a new factory for the processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &config.Config{
		DropLogs: false, // Default to forwarding logs
	}
}

func createLogsProcessor(
	ctx context.Context,
	settings processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	processorCfg, ok := cfg.(*config.Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %+v", cfg)
	}

	if err := processorCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	p, err := newProcessor(processorCfg, nextConsumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}

	return processorhelper.NewLogs(
		ctx,
		settings,
		cfg,
		nextConsumer,
		p.processLogs,
		processorhelper.WithCapabilities(p.Capabilities()),
		processorhelper.WithStart(p.Start),
		processorhelper.WithShutdown(p.Shutdown),
	)
}

