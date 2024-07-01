// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type aggregateOnAttributesArguments struct {
	Type       string
	Attributes ottl.Optional[[]string]
}

func newAggregateOnAttributesFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("aggregate_on_attributes", &aggregateOnAttributesArguments{}, createAggregateOnAttributesFunction)
}

func createAggregateOnAttributesFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*aggregateOnAttributesArguments)

	if !ok {
		return nil, fmt.Errorf("AggregateOnAttributesFactory args must be of type *AggregateOnAttributesArguments")
	}

	t, err := aggregateutil.ConvertToAggregationType(args.Type)
	if err != nil {
		return nil, fmt.Errorf("aggregation function invalid: %s", err.Error())
	}

	return AggregateOnAttributes(t, args.Attributes)
}

func AggregateOnAttributes(aggregationType aggregateutil.AggregationType, attributes ottl.Optional[[]string]) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		if metric.Type() == pmetric.MetricTypeSummary {
			return nil, fmt.Errorf("aggregation function is not supported for Summary metrics")
		}

		ag := aggregateutil.AggGroups{}
		aggregateutil.FilterAttrs(metric, attributes.Get())
		newMetric := pmetric.NewMetric()
		aggregateutil.CopyMetricDetails(metric, newMetric)
		aggregateutil.GroupDataPoints(metric, &ag)
		aggregateutil.MergeDataPoints(newMetric, aggregationType, ag)
		newMetric.MoveTo(metric)

		return nil, nil
	}, nil
}
