// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

var _ consumer.Logs = &logStatements{}

type logStatements struct {
	ottl.StatementSequence[ottllog.TransformContext]
	expr.BoolExpr[ottllog.TransformContext]
}

func (l logStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (l logStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				tCtx := ottllog.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource(), slogs, rlogs)
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

type LogParserCollection ottl.ParserCollection[ContextStatements, consumer.Logs]

type LogParserCollectionOption ottl.ParserCollectionOption[ContextStatements, consumer.Logs]

func WithLogParser(functions map[string]ottl.Factory[ottllog.TransformContext]) LogParserCollectionOption {
	return func(pc *ottl.ParserCollection[ContextStatements, consumer.Logs]) error {
		logParser, err := ottllog.NewParser(functions, pc.Settings, ottllog.WithPathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottllog.PathContextName, &logParser, convertLogStatements)(pc)
	}
}

func convertLogStatements(pc *ottl.ParserCollection[ContextStatements, consumer.Logs], _ *ottl.Parser[ottllog.TransformContext], _ string, statements ContextStatements, parsedStatements []*ottl.Statement[ottllog.TransformContext]) (consumer.Logs, error) {
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForLog, statements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardLogFuncs())
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	lStatements := ottllog.NewStatementSequence(parsedStatements, pc.Settings, ottllog.WithStatementSequenceErrorMode(pc.ErrorMode))
	return logStatements{lStatements, globalExpr}, nil
}

func WithLogErrorMode(errorMode ottl.ErrorMode) LogParserCollectionOption {
	return LogParserCollectionOption(ottl.WithParserCollectionErrorMode[ContextStatements, consumer.Logs](errorMode))
}

func NewLogParserCollection(settings component.TelemetrySettings, options ...LogParserCollectionOption) (*LogParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[ContextStatements, consumer.Logs]{
		withCommonContextParsers[consumer.Logs](),
		ottl.EnableParserCollectionModifiedStatementLogging[ContextStatements, consumer.Logs](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[ContextStatements, consumer.Logs](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	lpc := LogParserCollection(*pc)
	return &lpc, nil
}

func (lpc *LogParserCollection) ParseContextStatements(contextStatements ContextStatements) (consumer.Logs, error) {
	pc := ottl.ParserCollection[ContextStatements, consumer.Logs](*lpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements)
}
