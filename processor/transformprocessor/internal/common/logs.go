// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type LogsConsumer interface {
	Context() ContextID
	ConsumeLogs(ctx context.Context, ld plog.Logs, cache *pcommon.Map) error
}

type logStatements struct {
	ottl.StatementSequence[ottllog.TransformContext]
	expr.BoolExpr[ottllog.TransformContext]
}

func (l logStatements) Context() ContextID {
	return Log
}

func (l logStatements) ConsumeLogs(ctx context.Context, ld plog.Logs, cache *pcommon.Map) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				tCtx := ottllog.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource(), slogs, rlogs, ottllog.WithCache(cache))
				condition, err := l.BoolExpr.Eval(ctx, tCtx)
				if err != nil {
					return err
				}
				if condition {
					err := l.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

type LogParserCollection ottl.ParserCollection[LogsConsumer]

type LogParserCollectionOption ottl.ParserCollectionOption[LogsConsumer]

func WithLogParser(functions map[string]ottl.Factory[ottllog.TransformContext]) LogParserCollectionOption {
	return func(pc *ottl.ParserCollection[LogsConsumer]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottllog.ContextName, &logParser, convertLogStatements)(pc)
	}
}

func WithLogErrorMode(errorMode ottl.ErrorMode) LogParserCollectionOption {
	return LogParserCollectionOption(ottl.WithParserCollectionErrorMode[LogsConsumer](errorMode))
}

func NewLogParserCollection(settings component.TelemetrySettings, options ...LogParserCollectionOption) (*LogParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[LogsConsumer]{
		withCommonContextParsers[LogsConsumer](),
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

func convertLogStatements(pc *ottl.ParserCollection[LogsConsumer], _ *ottl.Parser[ottllog.TransformContext], _ string, statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottllog.TransformContext]) (LogsConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForLog, contextStatements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardLogFuncs())
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	lStatements := ottllog.NewStatementSequence(parsedStatements, pc.Settings, ottllog.WithStatementSequenceErrorMode(pc.ErrorMode))
	return logStatements{lStatements, globalExpr}, nil
}

func (lpc *LogParserCollection) ParseContextStatements(contextStatements ContextStatements) (LogsConsumer, error) {
	pc := ottl.ParserCollection[LogsConsumer](*lpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements)
}
