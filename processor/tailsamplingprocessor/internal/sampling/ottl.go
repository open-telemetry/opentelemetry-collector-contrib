// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type ottlConditionFilter struct {
	sampleSpanExpr      expr.BoolExpr[ottlspan.TransformContext]
	sampleSpanEventExpr expr.BoolExpr[ottlspanevent.TransformContext]
	errorMode           ottl.ErrorMode
	logger              *zap.Logger
}

var _ PolicyEvaluator = (*ottlConditionFilter)(nil)

// NewOTTLConditionFilter looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLConditionFilter(settings component.TelemetrySettings, spanConditions, spanEventConditions []string, errMode ottl.ErrorMode) (PolicyEvaluator, error) {
	filter := &ottlConditionFilter{
		errorMode: errMode,
		logger:    settings.Logger,
	}

	var err error

	if len(spanConditions) == 0 && len(spanEventConditions) == 0 {
		return nil, errors.New("expected at least one OTTL condition to filter on")
	}

	if len(spanConditions) > 0 {
		if filter.sampleSpanExpr, err = filterottl.NewBoolExprForSpan(spanConditions, filterottl.StandardSpanFuncs(), errMode, settings); err != nil {
			return nil, err
		}
	}

	if len(spanEventConditions) > 0 {
		if filter.sampleSpanEventExpr, err = filterottl.NewBoolExprForSpanEvent(spanEventConditions, filterottl.StandardSpanEventFuncs(), errMode, settings); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

func (ocf *ottlConditionFilter) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	ocf.logger.Debug("Evaluating with OTTL conditions filter", zap.String("traceID", traceID.String()))

	if ocf.sampleSpanExpr == nil && ocf.sampleSpanEventExpr == nil {
		return NotSampled, nil
	}

	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		resource := rs.Resource()
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			scope := ss.Scope()
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				var (
					ok  bool
					err error
				)

				// Now we reach span level and begin evaluation with parsed expr.
				// The evaluation will break when:
				// 1. error happened.
				// 2. "Sampled" decision made.
				// Otherwise, it will keep evaluating and finally exit with "NotSampled" decision.

				// Span evaluation
				if ocf.sampleSpanExpr != nil {
					ok, err = ocf.sampleSpanExpr.Eval(ctx, ottlspan.NewTransformContext(span, scope, resource))
					if err != nil {
						return Error, err
					}
					if ok {
						return Sampled, nil
					}
				}

				// Span event evaluation
				if ocf.sampleSpanEventExpr != nil {
					spanEvents := span.Events()
					for l := 0; l < spanEvents.Len(); l++ {
						ok, err = ocf.sampleSpanEventExpr.Eval(ctx, ottlspanevent.NewTransformContext(spanEvents.At(l), span, scope, resource))
						if err != nil {
							return Error, err
						}
						if ok {
							return Sampled, nil
						}
					}
				}
			}
		}
	}
	return NotSampled, nil
}
