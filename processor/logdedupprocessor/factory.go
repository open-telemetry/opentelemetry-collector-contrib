// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
)

func parseConditions(conditions []string, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottllog.TransformContext], error) {
	parser, err := ottllog.NewParser(filterottl.StandardLogFuncs(), set, ottllog.EnablePathContextNames())
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection[*ottl.ConditionSequence[*ottllog.TransformContext]](
		set,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ConditionSequence[*ottllog.TransformContext]](true),
		ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[*ottl.ConditionSequence[*ottllog.TransformContext]], _ ottl.ConditionsGetter, parsed []*ottl.Condition[*ottllog.TransformContext]) (*ottl.ConditionSequence[*ottllog.TransformContext], error) {
				c := ottllog.NewConditionSequence(parsed, set, ottllog.WithConditionSequenceErrorMode(ottl.PropagateError))
				return &c, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	return pc.ParseConditionsWithContext(ottllog.ContextName, ottl.NewConditionsGetter(conditions), true)
}

// NewFactory creates a new factory for the processor.
func NewFactory() processor.Factory {
	return xprocessor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xprocessor.WithLogs(createLogsProcessor, metadata.LogsStability),
		xprocessor.WithDeprecatedTypeAlias(metadata.DeprecatedType),
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
		conditions, err := parseConditions(processorCfg.Conditions, settings.TelemetrySettings)
		if err != nil {
			return nil, fmt.Errorf("invalid condition: %w", err)
		}
		processor.conditions = conditions
	}

	return processor, nil
}
