// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type aggregateLabelValueArguments struct {
	Type         common.AggregationType
	Label        string
	ValueSet     map[string]bool
	NewLabelName string
}

func newAggregateLabelValueFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("aggregate_label_value", &aggregateLabelValueArguments{}, createAggregateLabelFunction)
}

func createAggregateLabelValueFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*aggregateLabelValueArguments)

	if !ok {
		return nil, fmt.Errorf("AggregateLabelValueFactory args must be of type *AggregateLabelValueArguments")
	}

	if !args.Type.IsValid() {
		return nil, fmt.Errorf("AggregateLabelValue accepts one of the following functions: min/max/mean/sum")
	}

	return AggregateLabelValue(args.Type, args.Label, args.ValueSet, args.NewLabelName)
}

func AggregateLabelValue(aggregationType common.AggregationType, label string, valueSetMap map[string]bool, newLabelName string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		rangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
			val, ok := attrs.Get(label)
			if !ok {
				return true
			}

			if _, ok := valueSetMap[val.Str()]; ok {
				val.SetStr(newLabelName)
			}
			return true
		})
		newMetric := pmetric.NewMetric()
		copyMetricDetails(metric, newMetric)
		ag := groupDataPoints(metric)
		mergeDataPoints(newMetric, aggregationType, ag)
		newMetric.MoveTo(metric)

		return nil, nil
	}, nil
}
