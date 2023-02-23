// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func ParseSpan(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottlspan.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottlspan.NewParser(functions[ottlspan.TransformContext](), set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseSpanEvent(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottlspanevent.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottlspanevent.NewParser(functions[ottlspanevent.TransformContext](), set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseLog(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottllog.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottllog.NewParser(functions[ottllog.TransformContext](), set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseMetric(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottlmetric.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottlmetric.NewParser(metricFunctions(), set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseDataPoint(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottldatapoint.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser, err := ottldatapoint.NewParser(functions[ottldatapoint.TransformContext](), set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func conditionsToStatements(conditions []string) []string {
	statements := make([]string, len(conditions))
	for i, condition := range conditions {
		statements[i] = "drop() where " + condition
	}
	return statements
}

type statementExpr[K any] struct {
	statement *ottl.Statement[K]
}

func (se statementExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	_, ret, err := se.statement.Execute(ctx, tCtx)
	return ret, err
}

func statementsToExpr[K any](statements []*ottl.Statement[K]) expr.BoolExpr[K] {
	var rets []expr.BoolExpr[K]
	for _, statement := range statements {
		rets = append(rets, statementExpr[K]{statement: statement})
	}
	return expr.Or(rets...)
}

func functions[K any]() map[string]interface{} {
	return map[string]interface{}{
		"TraceID":     ottlfuncs.TraceID[K],
		"SpanID":      ottlfuncs.SpanID[K],
		"IsMatch":     ottlfuncs.IsMatch[K],
		"Concat":      ottlfuncs.Concat[K],
		"Split":       ottlfuncs.Split[K],
		"Int":         ottlfuncs.Int[K],
		"ConvertCase": ottlfuncs.ConvertCase[K],
		"drop": func() (ottl.ExprFunc[K], error) {
			return func(context.Context, K) (interface{}, error) {
				return true, nil
			}, nil
		},
	}
}

func metricFunctions() map[string]interface{} {
	funcs := functions[ottlmetric.TransformContext]()
	funcs["HasAttrKeyOnDatapoint"] = hasAttributeKeyOnDatapoint
	funcs["HasAttrOnDatapoint"] = hasAttributeOnDatapoint

	return funcs
}

func hasAttributeOnDatapoint(key string, expectedVal string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(ctx context.Context, tCtx ottlmetric.TransformContext) (interface{}, error) {
		return checkDataPoints(tCtx, key, &expectedVal)
	}, nil
}

func hasAttributeKeyOnDatapoint(key string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(ctx context.Context, tCtx ottlmetric.TransformContext) (interface{}, error) {
		return checkDataPoints(tCtx, key, nil)
	}, nil
}

func checkDataPoints(tCtx ottlmetric.TransformContext, key string, expectedVal *string) (interface{}, error) {
	metric := tCtx.GetMetric()
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		return checkNumberDataPointSlice(metric.Sum().DataPoints(), key, expectedVal), nil
	case pmetric.MetricTypeGauge:
		return checkNumberDataPointSlice(metric.Gauge().DataPoints(), key, expectedVal), nil
	case pmetric.MetricTypeHistogram:
		return checkHistogramDataPointSlice(metric.Histogram().DataPoints(), key, expectedVal), nil
	case pmetric.MetricTypeExponentialHistogram:
		return checkExponentialHistogramDataPointSlice(metric.ExponentialHistogram().DataPoints(), key, expectedVal), nil
	case pmetric.MetricTypeSummary:
		return checkSummaryDataPointSlice(metric.Summary().DataPoints(), key, expectedVal), nil
	}
	return nil, fmt.Errorf("unknown metric type")
}

func checkNumberDataPointSlice(dps pmetric.NumberDataPointSlice, key string, expectedVal *string) bool {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		value, ok := dp.Attributes().Get(key)
		if ok {
			if expectedVal != nil {
				return value.Str() == *expectedVal
			}
			return true
		}
	}
	return false
}

func checkHistogramDataPointSlice(dps pmetric.HistogramDataPointSlice, key string, expectedVal *string) bool {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		value, ok := dp.Attributes().Get(key)
		if ok {
			if expectedVal != nil {
				return value.Str() == *expectedVal
			}
			return true
		}
	}
	return false
}

func checkExponentialHistogramDataPointSlice(dps pmetric.ExponentialHistogramDataPointSlice, key string, expectedVal *string) bool {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		value, ok := dp.Attributes().Get(key)
		if ok {
			if expectedVal != nil {
				return value.Str() == *expectedVal
			}
			return true
		}
	}
	return false
}

func checkSummaryDataPointSlice(dps pmetric.SummaryDataPointSlice, key string, expectedVal *string) bool {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		value, ok := dp.Attributes().Get(key)
		if ok {
			if expectedVal != nil {
				return value.Str() == *expectedVal
			}
			return true
		}
	}
	return false
}
