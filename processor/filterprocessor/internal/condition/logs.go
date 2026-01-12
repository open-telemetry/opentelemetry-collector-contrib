// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

type LogsConsumer interface {
	Context() ContextID
	ConsumeLogs(ctx context.Context, ld plog.Logs) error
}

type logConditions struct {
	expr.BoolExpr[*ottllog.TransformContext]
}

func (logConditions) Context() ContextID {
	return Log
}

func (l logConditions) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var condErr error
	ld.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
		rlogs.ScopeLogs().RemoveIf(func(slogs plog.ScopeLogs) bool {
			slogs.LogRecords().RemoveIf(func(log plog.LogRecord) bool {
				tCtx := ottllog.NewTransformContextPtr(rlogs, slogs, log)
				cond, err := l.Eval(ctx, tCtx)
				tCtx.Close()
				if err != nil {
					condErr = multierr.Append(condErr, err)
					return false
				}
				return cond
			})
			return slogs.LogRecords().Len() == 0
		})
		return rlogs.ScopeLogs().Len() == 0
	})

	if ld.ResourceLogs().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type LogParserCollection ottl.ParserCollection[LogsConsumer]

type LogParserCollectionOption ottl.ParserCollectionOption[LogsConsumer]

func WithLogParser(functions map[string]ottl.Factory[*ottllog.TransformContext]) LogParserCollectionOption {
	return func(pc *ottl.ParserCollection[LogsConsumer]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottllog.ContextName, &logParser, ottl.WithConditionConverter(convertLogConditions))(pc)
	}
}

func WithLogErrorMode(errorMode ottl.ErrorMode) LogParserCollectionOption {
	return LogParserCollectionOption(ottl.WithParserCollectionErrorMode[LogsConsumer](errorMode))
}

func WithLogCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) LogParserCollectionOption {
	return LogParserCollectionOption(withCommonParsers[LogsConsumer](functions))
}

func NewLogParserCollection(settings component.TelemetrySettings, options ...LogParserCollectionOption) (*LogParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[LogsConsumer]{
		ottl.EnableParserCollectionModifiedPathsLogging[LogsConsumer](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[LogsConsumer](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	lpc := LogParserCollection(*pc)
	return &lpc, nil
}

func convertLogConditions(pc *ottl.ParserCollection[LogsConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottllog.TransformContext]) (LogsConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}

	errorMode := getErrorMode(pc, contextConditions)
	lConditions := ottllog.NewConditionSequence(parsedConditions, pc.Settings, ottllog.WithConditionSequenceErrorMode(errorMode))
	return logConditions{&lConditions}, nil
}

func (lpc *LogParserCollection) ParseContextConditions(contextConditions ContextConditions) (LogsConsumer, error) {
	pc := ottl.ParserCollection[LogsConsumer](*lpc)
	if contextConditions.Context != "" {
		return pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
	}
	return pc.ParseConditions(contextConditions)
}
