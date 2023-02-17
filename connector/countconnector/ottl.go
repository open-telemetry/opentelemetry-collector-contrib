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

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newSpanParser(logger *zap.Logger) (ottl.Parser[ottlspan.TransformContext], error) {
	return ottlspan.NewParser(
		ottlFunctions[ottlspan.TransformContext](),
		component.TelemetrySettings{Logger: logger})
}
func newSpanEventParser(logger *zap.Logger) (ottl.Parser[ottlspanevent.TransformContext], error) {
	return ottlspanevent.NewParser(
		ottlFunctions[ottlspanevent.TransformContext](),
		component.TelemetrySettings{Logger: logger})
}
func newMetricParser(logger *zap.Logger) (ottl.Parser[ottlmetric.TransformContext], error) {
	return ottlmetric.NewParser(
		ottlFunctions[ottlmetric.TransformContext](),
		component.TelemetrySettings{Logger: logger})
}
func newDataPointParser(logger *zap.Logger) (ottl.Parser[ottldatapoint.TransformContext], error) {
	return ottldatapoint.NewParser(
		ottlFunctions[ottldatapoint.TransformContext](),
		component.TelemetrySettings{Logger: logger})
}
func newLogParser(logger *zap.Logger) (ottl.Parser[ottllog.TransformContext], error) {
	return ottllog.NewParser(
		ottlFunctions[ottllog.TransformContext](),
		component.TelemetrySettings{Logger: logger})
}

func parseConditions[K any](parser ottl.Parser[K], conditions []string) (expr.BoolExpr[K], error) {
	statmentsStr := conditionsToStatements(conditions)
	statements, err := parser.ParseStatements(statmentsStr)
	if err != nil {
		return nil, err
	}
	return statementsToExpr(statements), nil
}

func conditionsToStatements(conditions []string) []string {
	statements := make([]string, len(conditions))
	for i, condition := range conditions {
		statements[i] = "noop() where " + condition
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

func ottlFunctions[K any]() map[string]interface{} {
	return map[string]interface{}{
		"TraceID":     ottlfuncs.TraceID[K],
		"SpanID":      ottlfuncs.SpanID[K],
		"IsMatch":     ottlfuncs.IsMatch[K],
		"Concat":      ottlfuncs.Concat[K],
		"Split":       ottlfuncs.Split[K],
		"Int":         ottlfuncs.Int[K],
		"Substring":   ottlfuncs.Substring[K],
		"ConvertCase": ottlfuncs.ConvertCase[K],
		"noop": func() (ottl.ExprFunc[K], error) {
			return func(context.Context, K) (interface{}, error) {
				return true, nil
			}, nil
		},
	}
}
