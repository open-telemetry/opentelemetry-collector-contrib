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
	"go.uber.org/zap"
)

type ContextID string

type ottlStatementFilter struct {
	sampleSpanExpr      expr.BoolExpr[ottlspan.TransformContext]
	sampleSpanEventExpr expr.BoolExpr[ottlspanevent.TransformContext]
	logger              *zap.Logger
}

var _ PolicyEvaluator = (*ottlStatementFilter)(nil)

// NewOTTLStatementFilter looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLStatementFilter(logger *zap.Logger, spanStatements, spanEventStatements []string) PolicyEvaluator {
	sampleSpanExpr, _ := filterottl.NewBoolExprForSpan(spanStatements, filterottl.StandardSpanFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
	sampleSpanEventExpr, _ := filterottl.NewBoolExprForSpanEvent(spanEventStatements, filterottl.StandardSpanEventFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})

	return &ottlStatementFilter{
		sampleSpanExpr:      sampleSpanExpr,
		sampleSpanEventExpr: sampleSpanEventExpr,
		logger:              logger,
	}
}

func (osf *ottlStatementFilter) Evaluate(ctx context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	osf.logger.Debug("Evaluating spans with OTTL statement filter")

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
				ok, err := osf.sampleSpanExpr.Eval(ctx, ottlspan.NewTransformContext(span, scope, resource))
				if err != nil {
					osf.logger.Error("failed processing traces", zap.Error(err))
					return NotSampled, err
				}
				if ok {
					return Sampled, nil
				}

				spanEvents := span.Events()
				for l := 0; l < spanEvents.Len(); l++ {
					ok, err = osf.sampleSpanEventExpr.Eval(ctx, ottlspanevent.NewTransformContext(spanEvents.At(l), span, scope, resource))
					if err != nil {
						osf.logger.Error("failed processing traces", zap.Error(err))
						return NotSampled, err
					}
					if ok {
						return Sampled, nil
					}
				}
			}
		}
	}
	return NotSampled, nil
}
