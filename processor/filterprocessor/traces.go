// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterSpanProcessor struct {
	consumers         []common.TracesConsumer
	skipSpanExpr      expr.BoolExpr[ottlspan.TransformContext]
	skipSpanEventExpr expr.BoolExpr[ottlspanevent.TransformContext]
	telemetry         *filterTelemetry
	logger            *zap.Logger
}

func newFilterSpansProcessor(set processor.Settings, cfg *Config) (*filterSpanProcessor, error) {
	var err error
	fsp := &filterSpanProcessor{
		logger: set.Logger,
	}

	fpt, err := newFilterTelemetry(set, pipeline.SignalTraces)
	if err != nil {
		return nil, fmt.Errorf("error creating filter processor telemetry: %w", err)
	}
	fsp.telemetry = fpt

	if len(cfg.TraceConditions) > 0 {
		pc, collectionErr := common.NewTraceParserCollection(set.TelemetrySettings, common.WithSpanParser(filterottl.StandardSpanFuncs()), common.WithSpanEventParser(filterottl.StandardSpanEventFuncs()))
		if collectionErr != nil {
			return nil, collectionErr
		}
		var errors error
		for _, cs := range cfg.TraceConditions {
			metricConsumer, parseErr := pc.ParseContextConditions(cs)
			errors = multierr.Append(errors, parseErr)
			fsp.consumers = append(fsp.consumers, metricConsumer)
		}
		if errors != nil {
			return nil, errors
		}
	}

	if cfg.Traces.SpanConditions != nil || cfg.Traces.SpanEventConditions != nil {
		if cfg.Traces.SpanConditions != nil {
			fsp.skipSpanExpr, err = filterottl.NewBoolExprForSpan(cfg.Traces.SpanConditions, filterottl.StandardSpanFuncs(), cfg.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, err
			}
		}
		if cfg.Traces.SpanEventConditions != nil {
			fsp.skipSpanEventExpr, err = filterottl.NewBoolExprForSpanEvent(cfg.Traces.SpanEventConditions, filterottl.StandardSpanEventFuncs(), cfg.ErrorMode, set.TelemetrySettings)
			if err != nil {
				return nil, err
			}
		}
		return fsp, nil
	}

	fsp.skipSpanExpr, err = filterspan.NewSkipExpr(&cfg.Spans)
	if err != nil {
		return nil, err
	}

	includeMatchType, excludeMatchType := "[None]", "[None]"
	if cfg.Spans.Include != nil {
		includeMatchType = string(cfg.Spans.Include.MatchType)
	}

	if cfg.Spans.Exclude != nil {
		excludeMatchType = string(cfg.Spans.Exclude.MatchType)
	}

	set.Logger.Info(
		"Span filter configured",
		zap.String("[Include] match_type", includeMatchType),
		zap.String("[Exclude] match_type", excludeMatchType),
	)

	return fsp, nil
}

func (fsp *filterSpanProcessor) processExprs(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	var errors error
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		resource := rs.Resource()
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			scope := ss.Scope()
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				if fsp.skipSpanExpr != nil {
					skip, err := fsp.skipSpanExpr.Eval(ctx, ottlspan.NewTransformContext(span, scope, resource, ss, rs))
					if err != nil {
						errors = multierr.Append(errors, err)
						return false
					}
					if skip {
						return true
					}
				}
				if fsp.skipSpanEventExpr != nil {
					span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
						skip, err := fsp.skipSpanEventExpr.Eval(ctx, ottlspanevent.NewTransformContext(spanEvent, span, scope, resource, ss, rs))
						if err != nil {
							errors = multierr.Append(errors, err)
							return false
						}
						return skip
					})
				}
				return false
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
	return td, errors
}

func (fsp *filterSpanProcessor) processConditions(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	var errors error
	for _, consumer := range fsp.consumers {
		err := consumer.ConsumeTraces(ctx, td)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
	}
	return td, errors
}

// processTraces filters the given spans of a traces based off the filterSpanProcessor's filters.
func (fsp *filterSpanProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if fsp.skipSpanExpr == nil && fsp.skipSpanEventExpr == nil {
		return td, nil
	}

	spanCountBeforeFilters := td.SpanCount()

	var errors error
	var processedTraces ptrace.Traces
	if len(fsp.consumers) > 0 {
		processedTraces, errors = fsp.processConditions(ctx, td)
	} else {
		processedTraces, errors = fsp.processExprs(ctx, td)
	}

	spanCountAfterFilters := processedTraces.SpanCount()
	fsp.telemetry.record(ctx, int64(spanCountBeforeFilters-spanCountAfterFilters))

	if errors != nil {
		fsp.logger.Error("failed processing traces", zap.Error(errors))
		return processedTraces, errors
	}
	if processedTraces.ResourceSpans().Len() == 0 {
		return processedTraces, processorhelper.ErrSkipProcessingData
	}
	return processedTraces, nil
}
