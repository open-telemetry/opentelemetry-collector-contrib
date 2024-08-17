// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

//import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type filterDatapointsArguments struct {
	Attributes map[string]string
}

func newFilterDatapointsFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("filter_datapoints", &filterDatapointsArguments{}, createFilterDatapointsFunction)
}

func createFilterDatapointsFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*filterDatapointsArguments)

	if !ok {
		return nil, fmt.Errorf("createFilterDatapointsFunction args must be of type *filterDatapointsArguments")
	}

	return filterDatapoints(args.Attributes)
}

func filterDatapoints(attributes map[string]string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(ctx context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		// Get the current metric from the context
		cur := tCtx.GetMetric()
		// Get the datapoints from the current metric
		var dps pmetric.NumberDataPointSlice
		switch cur.Type() {
		case pmetric.MetricTypeGauge:
			dps = cur.Gauge().DataPoints()
		case pmetric.MetricTypeSum:
			dps = cur.Sum().DataPoints()
		default:
			// unsupported metric type
			return nil, nil
		}
		// Remove datapoints that do not match any of the key value pairs in attributes
		dps.RemoveIf(func(point pmetric.NumberDataPoint) bool {
			pointAttr := point.Attributes()
			for key, val := range attributes {
				pointAttrVal, hasKey := pointAttr.Get(key)
				if !hasKey || pointAttrVal.Str() != val {
					return true
				}
			}
			return false
		})

		return nil, nil
	}, nil
}
