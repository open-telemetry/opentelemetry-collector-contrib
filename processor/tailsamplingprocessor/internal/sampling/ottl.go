// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type ottlStatementFilter struct {
	sampleSpanExpr      expr.BoolExpr[ottlspan.TransformContext]
	sampleSpanEventExpr expr.BoolExpr[ottlspanevent.TransformContext]
	ErrorMode           ottl.ErrorMode
	logger              *zap.Logger
}

var _ PolicyEvaluator = (*ottlStatementFilter)(nil)

// evalFuncChain contains eval functions for:
// 1. span expression
// 2. span event expression
var evalFuncChain = []func(ctx context.Context, osf *ottlStatementFilter, span ptrace.Span, scope pcommon.InstrumentationScope, resource pcommon.Resource) (Decision, error){
	evalSpan,
	evalSpanEvent,
}

// NewOTTLStatementFilter looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLStatementFilter(logger *zap.Logger, spanStatements, spanEventStatements []string, errMode ottl.ErrorMode) (PolicyEvaluator, error) {
	filter := &ottlStatementFilter{
		ErrorMode: errMode,
		logger:    logger,
	}
	var err error
	if filter.sampleSpanExpr, err = filterottl.NewBoolExprForSpan(spanStatements, filterottl.StandardSpanFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
		return nil, err
	}
	if filter.sampleSpanEventExpr, err = filterottl.NewBoolExprForSpanEvent(spanEventStatements, filterottl.StandardSpanEventFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
		return nil, err
	}

	return filter, nil
}

func (osf *ottlStatementFilter) Evaluate(ctx context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	osf.logger.Debug("Evaluating spans with OTTL statement filter")

	if osf.sampleSpanExpr == nil && osf.sampleSpanEventExpr == nil {
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
				// Now we reach span level and execute each eval function.
				// The execution of function chain will break when:
				// 1. error happened.
				// 2. "Sampled" decision made.
				// Otherwise, it will keep executing and finally exit with "NotSampled" decision.
				for l := range evalFuncChain {
					decision, err := evalFuncChain[l](ctx, osf, ss.Spans().At(k), scope, resource)
					if err != nil {
						osf.logger.Error("failed processing traces", zap.Error(err))
						return Error, err
					}
					if decision == Sampled {
						return Sampled, nil
					}
				}
			}
		}
	}
	return NotSampled, nil
}

// evalSpan wrap expr.BoolExpr[ottlspan.TransformContext].Eval() and transform bool result into a sampling decision.
func evalSpan(ctx context.Context, osf *ottlStatementFilter, span ptrace.Span, scope pcommon.InstrumentationScope, resource pcommon.Resource) (Decision, error) {
	ok, err := osf.sampleSpanExpr.Eval(ctx, ottlspan.NewTransformContext(span, scope, resource))
	if err != nil {
		return Error, err
	}
	if ok {
		return Sampled, nil
	}
	return NotSampled, nil
}

// evalSpan wrap expr.BoolExpr[ottlspanevent.TransformContext].Eval() and transform bool result into a sampling decision.
func evalSpanEvent(ctx context.Context, osf *ottlStatementFilter, span ptrace.Span, scope pcommon.InstrumentationScope, resource pcommon.Resource) (Decision, error) {
	spanEvents := span.Events()
	for l := 0; l < spanEvents.Len(); l++ {
		ok, err := osf.sampleSpanEventExpr.Eval(ctx, ottlspanevent.NewTransformContext(spanEvents.At(l), span, scope, resource))
		if err != nil {
			return Error, err
		}
		if ok {
			return Sampled, nil
		}
	}
	return NotSampled, nil
}
