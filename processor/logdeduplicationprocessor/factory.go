// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdeduplicationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdeduplicationprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdeduplicationprocessor/internal/metadata"
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
func createLogsProcessor(_ context.Context, params processor.Settings, cfg component.Config, consumer consumer.Logs) (processor.Logs, error) {
	processorCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %+v", cfg)
	}

	return newProcessor(processorCfg, consumer, params.Logger)
}
