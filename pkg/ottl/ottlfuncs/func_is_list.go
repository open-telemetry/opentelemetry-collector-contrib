// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsListArguments[K any] struct {
	Target ottl.Getter[K]
}

func NewIsListFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsList", &IsListArguments[K]{}, createIsListFunction[K])
}

func createIsListFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsListArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsListFactory args must be of type *IsListArguments[K]")
	}

	return isList(args.Target), nil
}

// nolint:errorlint
func isList[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return false, err
		}

		switch valType := val.(type) {

		case pcommon.Slice:
			return true, nil
		case pcommon.Value:
			switch valType.Type() {
			case pcommon.ValueTypeSlice:
				return true, nil
			}
			return false, nil

		case plog.LogRecordSlice:
			return true, nil
		case plog.ResourceLogsSlice:
			return true, nil
		case plog.ScopeLogsSlice:
			return true, nil
		case pmetric.ExemplarSlice:
			return true, nil
		case pmetric.ExponentialHistogramDataPointSlice:
			return true, nil
		case pmetric.HistogramDataPointSlice:
			return true, nil
		case pmetric.MetricSlice:
			return true, nil
		case pmetric.NumberDataPointSlice:
			return true, nil
		case pmetric.ResourceMetricsSlice:
			return true, nil
		case pmetric.ScopeMetricsSlice:
			return true, nil
		case pmetric.SummaryDataPointSlice:
			return true, nil
		case pmetric.SummaryDataPointValueAtQuantileSlice:
			return true, nil
		case ptrace.ResourceSpansSlice:
			return true, nil
		case ptrace.ScopeSpansSlice:
			return true, nil
		case ptrace.SpanEventSlice:
			return true, nil
		case ptrace.SpanLinkSlice:
			return true, nil
		case ptrace.SpanSlice:
			return true, nil
		}

		return false, nil
	}
}
