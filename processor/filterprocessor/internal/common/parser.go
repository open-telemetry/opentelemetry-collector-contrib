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

	"go.opentelemetry.io/collector/component"

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
	parser := ottlspan.NewParser(functions[ottlspan.TransformContext](), set)
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseSpanEvent(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottlspanevent.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser := ottlspanevent.NewParser(functions[ottlspanevent.TransformContext](), set)
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseLog(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottllog.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser := ottllog.NewParser(functions[ottllog.TransformContext](), set)
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseMetric(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottlmetric.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser := ottlmetric.NewParser(functions[ottlmetric.TransformContext](), set)
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func ParseDataPoint(conditions []string, set component.TelemetrySettings) (expr.BoolExpr[ottldatapoint.TransformContext], error) {
	statmentsStr := conditionsToStatements(conditions)
	parser := ottldatapoint.NewParser(functions[ottldatapoint.TransformContext](), set)
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
