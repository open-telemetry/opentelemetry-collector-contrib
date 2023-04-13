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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type ContextID string

type ottlStatementFilter struct {
	sampleSpanExpr expr.BoolExpr[ottlspan.TransformContext]
	logger         *zap.Logger
}

var _ PolicyEvaluator = (*ottlStatementFilter)(nil)

// NewBoolExprForSpan creates a BoolExpr[ottlspan.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlspan.TransformContext.
// If a function named `drop` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForSpan(conditions []string, functions map[string]interface{}, errorMode ottl.ErrorMode, set component.TelemetrySettings) (expr.BoolExpr[ottlspan.TransformContext], error) {
	if _, ok := functions["sample"]; !ok {
		functions["sample"] = sample[ottllog.TransformContext]
	}

	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottlspan.NewParser(functions, set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	s := ottlspan.NewStatements(statements, set, ottlspan.WithErrorMode(errorMode))
	return &s, nil
}

func conditionsToStatements(conditions []string) []string {
	statements := make([]string, len(conditions))
	for i, condition := range conditions {
		statements[i] = "sample() where " + condition
	}
	return statements
}

// NewOTTLStatementFilter looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLStatementFilter(logger *zap.Logger, statements []string) PolicyEvaluator {
	sampleSpanExpr, _ := NewBoolExprForSpan(statements, standardFuncs[ottlspan.TransformContext](), ottl.IgnoreError, component.TelemetrySettings{Logger: zap.NewNop()})
	return &ottlStatementFilter{
		sampleSpanExpr: sampleSpanExpr,
		logger:         logger,
	}
}

func standardFuncs[K any]() map[string]interface{} {
	return map[string]interface{}{
		"TraceID":     ottlfuncs.TraceID[K],
		"SpanID":      ottlfuncs.SpanID[K],
		"IsMatch":     ottlfuncs.IsMatch[K],
		"Concat":      ottlfuncs.Concat[K],
		"Split":       ottlfuncs.Split[K],
		"Int":         ottlfuncs.Int[K],
		"ConvertCase": ottlfuncs.ConvertCase[K],
		"Substring":   ottlfuncs.Substring[K],
		"sample":      sample[K],
	}
}

func sample[K any]() (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return true, nil
	}, nil
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
			}
		}
	}
	return NotSampled, nil
}
