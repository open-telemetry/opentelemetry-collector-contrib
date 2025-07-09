// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type aggregateOnAttributeValueArguments struct {
	AggregationFunction string
	Attribute           string
	Values              []string
	NewValue            string
}

func newAggregateOnAttributeValueFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("aggregate_on_attribute_value", &aggregateOnAttributeValueArguments{}, createAggregateOnAttributeValueFunction)
}

func createAggregateOnAttributeValueFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*aggregateOnAttributeValueArguments)

	if !ok {
		return nil, errors.New("AggregateOnAttributeValueFactory args must be of type *AggregateOnAttributeValueArguments")
	}

	t, err := aggregateutil.ConvertToAggregationFunction(args.AggregationFunction)
	if err != nil {
		return nil, fmt.Errorf("invalid aggregation function: '%s', valid options: %s", err.Error(), aggregateutil.GetSupportedAggregationFunctionsList())
	}

	return AggregateOnAttributeValue(t, args.Attribute, args.Values, args.NewValue)
}

func AggregateOnAttributeValue(aggregationType aggregateutil.AggregationType, attribute string, values []string, newValue string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		aggregateutil.RangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
			val, ok := attrs.Get(attribute)
			if !ok {
				return true
			}

			for _, v := range values {
				if val.Str() == v {
					val.SetStr(newValue)
				}
			}
			return true
		})
		ag := aggregateutil.AggGroups{}
		newMetric := pmetric.NewMetric()
		aggregateutil.CopyMetricDetails(metric, newMetric)
		aggregateutil.GroupDataPoints(metric, &ag)
		aggregateutil.MergeDataPoints(newMetric, aggregationType, ag)
		newMetric.MoveTo(metric)

		return nil, nil
	}, nil
}
