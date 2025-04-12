// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type copyMetricArguments struct {
	Name        ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]]
	Description ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]]
	Unit        ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]]
}

func newCopyMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("copy_metric", &copyMetricArguments{}, createCopyMetricFunction)
}

func createCopyMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*copyMetricArguments)

	if !ok {
		return nil, errors.New("createCopyMetricFunction args must be of type *copyMetricArguments")
	}

	return copyMetric(args.Name, args.Description, args.Unit)
}

func copyMetric(name ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]], desc ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]], unit ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]]) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(ctx context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		cur := tCtx.GetMetric()
		metrics := tCtx.GetMetrics()
		newMetric := metrics.AppendEmpty()
		cur.CopyTo(newMetric)

		if !name.IsEmpty() {
			n, err := name.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			newMetric.SetName(n)
		}

		if !desc.IsEmpty() {
			d, err := desc.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			newMetric.SetDescription(d)
		}

		if !unit.IsEmpty() {
			u, err := unit.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			newMetric.SetUnit(u)
		}

		return nil, nil
	}, nil
}
