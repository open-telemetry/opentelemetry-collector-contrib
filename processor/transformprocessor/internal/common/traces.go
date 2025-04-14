// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type TracesConsumer interface {
	Context() ContextID
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
}

type traceStatements struct {
	ottl.StatementSequence[ottlspan.TransformContext]
	expr.BoolExpr[ottlspan.TransformContext]
}

func (t traceStatements) Context() ContextID {
	return Span
}

func (t traceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				tCtx := ottlspan.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource(), sspans, rspans)
				condition, err := t.Eval(ctx, tCtx)
				if err != nil {
					return err
				}
				if condition {
					err := t.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

type spanEventStatements struct {
	ottl.StatementSequence[ottlspanevent.TransformContext]
	expr.BoolExpr[ottlspanevent.TransformContext]
}

func (s spanEventStatements) Context() ContextID {
	return SpanEvent
}

func (s spanEventStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanEvents := span.Events()
				for n := 0; n < spanEvents.Len(); n++ {
					tCtx := ottlspanevent.NewTransformContext(spanEvents.At(n), span, sspans.Scope(), rspans.Resource(), sspans, rspans)
					condition, err := s.Eval(ctx, tCtx)
					if err != nil {
						return err
					}
					if condition {
						err := s.Execute(ctx, tCtx)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

type TraceParserCollection ottl.ParserCollection[TracesConsumer]

type TraceParserCollectionOption ottl.ParserCollectionOption[TracesConsumer]

func WithSpanParser(functions map[string]ottl.Factory[ottlspan.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspan.NewParser(functions, pc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspan.ContextName, &parser, ottl.WithStatementConverter[ottlspan.TransformContext, TracesConsumer](convertSpanStatements))(pc)
	}
}

func WithSpanEventParser(functions map[string]ottl.Factory[ottlspanevent.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspanevent.NewParser(functions, pc.Settings, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspanevent.ContextName, &parser, ottl.WithStatementConverter(convertSpanEventStatements))(pc)
	}
}

func WithTraceErrorMode(errorMode ottl.ErrorMode) TraceParserCollectionOption {
	return TraceParserCollectionOption(ottl.WithParserCollectionErrorMode[TracesConsumer](errorMode))
}

func NewTraceParserCollection(settings component.TelemetrySettings, options ...TraceParserCollectionOption) (*TraceParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[TracesConsumer]{
		withCommonContextParsers[TracesConsumer](),
		ottl.EnableParserCollectionModifiedPathsLogging[TracesConsumer](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[TracesConsumer](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	tpc := TraceParserCollection(*pc)
	return &tpc, nil
}

func convertSpanStatements(pc *ottl.ParserCollection[TracesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlspan.TransformContext]) (TracesConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlspan.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlspan.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForSpanWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardSpanFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	sStatements := ottlspan.NewStatementSequence(parsedStatements, pc.Settings, ottlspan.WithStatementSequenceErrorMode(errorMode))
	return traceStatements{sStatements, globalExpr}, nil
}

func convertSpanEventStatements(pc *ottl.ParserCollection[TracesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlspanevent.TransformContext]) (TracesConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlspanevent.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlspanevent.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForSpanEventWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardSpanEventFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	seStatements := ottlspanevent.NewStatementSequence(parsedStatements, pc.Settings, ottlspanevent.WithStatementSequenceErrorMode(errorMode))
	return spanEventStatements{seStatements, globalExpr}, nil
}

func (tpc *TraceParserCollection) ParseContextStatements(contextStatements ContextStatements) (TracesConsumer, error) {
	pc := ottl.ParserCollection[TracesConsumer](*tpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements)
}
