// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package throttlingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor/internal/metadata"
)

// NewFactory creates a new factory for the processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

// createLogsProcessor creates a log processor.
func createLogsProcessor(_ context.Context, settings processor.Settings, cfg component.Config, consumer consumer.Logs) (processor.Logs, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %+v", cfg)
	}
	if err := processorCfg.Validate(); err != nil {
		return nil, err
	}
	proc, err := newProcessor(processorCfg, consumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}
	return proc, nil
}
