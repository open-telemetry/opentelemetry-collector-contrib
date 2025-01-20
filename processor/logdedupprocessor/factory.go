// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
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

	processor, err := newProcessor(processorCfg, consumer, settings)
	if err != nil {
		return nil, fmt.Errorf("error creating processor: %w", err)
	}

	if len(processorCfg.Conditions) == 0 {
		processor.conditions = nil
	} else {
		conditions, err := filterottl.NewBoolExprForLog(
			processorCfg.Conditions,
			filterottl.StandardLogFuncs(),
			ottl.PropagateError,
			settings.TelemetrySettings,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid condition: %w", err)
		}
		processor.conditions = conditions
	}

	return processor, nil
}
