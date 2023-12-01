// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ consumer.Logs = &logStatements{}

type logStatements struct {
	ottl.StatementSequence[ottllog.TransformContext]
}

func (l logStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (l logStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) (err error) {
	ld.ResourceLogs().Range(func(_ int, rlogs plog.ResourceLogs) {
		rlogs.ScopeLogs().Range(func(_ int, slogs plog.ScopeLogs) {
			slogs.LogRecords().Range(func(_ int, lr plog.LogRecord) {
				if err != nil {
					return // Would use RangeWhile instead if available
				}
				tCtx := ottllog.NewTransformContext(lr, slogs.Scope(), rlogs.Resource())
				err = l.Execute(ctx, tCtx)
			})
		})
	})
	return
}

type LogParserCollection struct {
	parserCollection
	logParser ottl.Parser[ottllog.TransformContext]
}

type LogParserCollectionOption func(*LogParserCollection) error

func WithLogParser(functions map[string]ottl.Factory[ottllog.TransformContext]) LogParserCollectionOption {
	return func(lp *LogParserCollection) error {
		logParser, err := ottllog.NewParser(functions, lp.settings)
		if err != nil {
			return err
		}
		lp.logParser = logParser
		return nil
	}
}

func WithLogErrorMode(errorMode ottl.ErrorMode) LogParserCollectionOption {
	return func(lp *LogParserCollection) error {
		lp.errorMode = errorMode
		return nil
	}
}

func NewLogParserCollection(settings component.TelemetrySettings, options ...LogParserCollectionOption) (*LogParserCollection, error) {
	rp, err := ottlresource.NewParser(ResourceFunctions(), settings)
	if err != nil {
		return nil, err
	}
	sp, err := ottlscope.NewParser(ScopeFunctions(), settings)
	if err != nil {
		return nil, err
	}
	lpc := &LogParserCollection{
		parserCollection: parserCollection{
			settings:       settings,
			resourceParser: rp,
			scopeParser:    sp,
		},
	}

	for _, op := range options {
		err := op(lpc)
		if err != nil {
			return nil, err
		}
	}

	return lpc, nil
}

func (pc LogParserCollection) ParseContextStatements(contextStatements ContextStatements) (consumer.Logs, error) {
	switch contextStatements.Context {
	case Log:
		parsedStatements, err := pc.logParser.ParseStatements(contextStatements.Statements)
		if err != nil {
			return nil, err
		}
		lStatements := ottllog.NewStatementSequence(parsedStatements, pc.settings, ottllog.WithStatementSequenceErrorMode(pc.errorMode))
		return logStatements{lStatements}, nil
	default:
		statements, err := pc.parseCommonContextStatements(contextStatements)
		if err != nil {
			return nil, err
		}
		return statements, nil
	}
}
