// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type TracesConsumer interface {
	Context() ContextID
	ConsumeTraces(ctx context.Context, td ptrace.Traces) error
}

type traceConditions struct {
	expr.BoolExpr[*ottlspan.TransformContext]
}

func (traceConditions) Context() ContextID {
	return Span
}

func (t traceConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
				cond, err := t.Eval(ctx, tCtx)
				tCtx.Close()

				if err != nil {
					condErr = multierr.Append(condErr, err)
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
	expr.BoolExpr[*ottlspanevent.TransformContext]
}

func (spanEventConditions) Context() ContextID {
	return SpanEvent
}

func (s spanEventConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			sspans.Spans().RemoveIf(func(span ptrace.Span) bool {
				span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
					tCtx := ottlspanevent.NewTransformContextPtr(rspans, sspans, span, spanEvent)
					cond, err := s.Eval(ctx, tCtx)
					tCtx.Close()

					if err != nil {
						condErr = multierr.Append(condErr, err)
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

func WithSpanParser(functions map[string]ottl.Factory[*ottlspan.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspan.NewParser(functions, pc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspan.ContextName, &parser, ottl.WithConditionConverter(convertSpanConditions))(pc)
	}
}

func WithSpanEventParser(functions map[string]ottl.Factory[*ottlspanevent.TransformContext]) TraceParserCollectionOption {
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

func WithTraceCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) TraceParserCollectionOption {
	return TraceParserCollectionOption(withCommonParsers[TracesConsumer](functions))
}

func NewTraceParserCollection(settings component.TelemetrySettings, options ...TraceParserCollectionOption) (*TraceParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[TracesConsumer]{
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

func convertSpanConditions(pc *ottl.ParserCollection[TracesConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottlspan.TransformContext]) (TracesConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	sConditions := ottlspan.NewConditionSequence(parsedConditions, pc.Settings, ottlspan.WithConditionSequenceErrorMode(errorMode))
	return traceConditions{&sConditions}, nil
}

func convertSpanEventConditions(pc *ottl.ParserCollection[TracesConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottlspanevent.TransformContext]) (TracesConsumer, error) {
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
	if contextConditions.Context != "" {
		return pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
	}
	return pc.ParseConditions(contextConditions)
}
