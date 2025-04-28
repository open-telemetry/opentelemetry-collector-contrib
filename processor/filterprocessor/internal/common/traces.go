// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type TracesConsumer interface {
	Context() ContextID
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
}

type traceConditions struct {
	expr.BoolExpr[ottlspan.TransformContext]
}

func (t traceConditions) Context() ContextID {
	return Span
}

func (t traceConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		resource := rs.Resource()
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			scope := ss.Scope()
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				tCtx := ottlspan.NewTransformContext(span, scope, resource, ss, rs)
				cond, err := t.BoolExpr.Eval(ctx, tCtx)
				if err != nil {
					condErr = err
					return false
				}
				return cond
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type spanEventConditions struct {
	expr.BoolExpr[ottlspanevent.TransformContext]
}

func (s spanEventConditions) Context() ContextID {
	return SpanEvent
}

func (s spanEventConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		resource := rspans.Resource()
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			scope := sspans.Scope()
			sspans.Spans().RemoveIf(func(span ptrace.Span) bool {
				span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
					tCtx := ottlspanevent.NewTransformContext(spanEvent, span, scope, resource, sspans, rspans)
					cond, err := s.BoolExpr.Eval(ctx, tCtx)
					if err != nil {
						condErr = err
						return false
					}
					return cond
				})
				return false
			})
			return sspans.Spans().Len() == 0
		})
		return rspans.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type TraceParserCollection ottl.ParserCollection[TracesConsumer]

type TraceParserCollectionOption ottl.ParserCollectionOption[TracesConsumer]

func WithSpanParser(functions map[string]ottl.Factory[ottlspan.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspan.NewParser(functions, pc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspan.ContextName, &parser, ottl.WithConditionConverter[ottlspan.TransformContext, TracesConsumer](convertSpanConditions))(pc)
	}
}

func WithSpanEventParser(functions map[string]ottl.Factory[ottlspanevent.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspanevent.NewParser(functions, pc.Settings, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspanevent.ContextName, &parser, ottl.WithConditionConverter(convertSpanEventConditions))(pc)
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

func convertSpanConditions(pc *ottl.ParserCollection[TracesConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottlspan.TransformContext]) (TracesConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	sConditions := ottlspan.NewConditionSequence(parsedConditions, pc.Settings, ottlspan.WithConditionSequenceErrorMode(errorMode))
	return traceConditions{&sConditions}, nil
}

func convertSpanEventConditions(pc *ottl.ParserCollection[TracesConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottlspanevent.TransformContext]) (TracesConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	seConditions := ottlspanevent.NewConditionSequence(parsedConditions, pc.Settings, ottlspanevent.WithConditionSequenceErrorMode(errorMode))
	return spanEventConditions{&seConditions}, nil
}

func (tpc *TraceParserCollection) ParseContextConditions(contextConditions ContextConditions) (TracesConsumer, error) {
	pc := ottl.ParserCollection[TracesConsumer](*tpc)
	return pc.ParseConditions(contextConditions)
}
