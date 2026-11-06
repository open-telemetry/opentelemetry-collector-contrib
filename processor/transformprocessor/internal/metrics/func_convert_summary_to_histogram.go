// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertSummaryToHistogramArguments struct {
	StringAggTemp string
	Suffix        ottl.Optional[string]
}

func newConvertSummaryToHistogramFactory() ottl.Factory[*ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_summary_to_histogram", &convertSummaryToHistogramArguments{}, createConvertSummaryToHistogramFunction)
}

func createConvertSummaryToHistogramFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertSummaryToHistogramArguments)
	if !ok {
		return nil, errors.New("convertSummaryToHistogramFactory args must be of type *convertSummaryToHistogramArguments")
	}

	return convertSummaryToHistogram(args.StringAggTemp, args.Suffix)
}

func convertSummaryToHistogram(stringAggTemp string, suffix ottl.Optional[string]) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	// An empty suffix would give the new metric the same name as the Summary.
	metricNameSuffix := "_histogram"
	if !suffix.IsEmpty() && suffix.Get() != "" {
		metricNameSuffix = suffix.Get()
	}
	var aggTemp pmetric.AggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.AggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.AggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}

	return func(_ context.Context, tCtx *ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSummary {
			return nil, nil
		}

		histogramMetric := tCtx.GetMetrics().AppendEmpty()
		histogramMetric.SetDescription(metric.Description())
		histogramMetric.SetName(metric.Name() + metricNameSuffix)
		histogramMetric.SetUnit(metric.Unit())
		histogram := histogramMetric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(aggTemp)

		dps := metric.Summary().DataPoints()
		histogramDps := histogram.DataPoints()
		histogramDps.EnsureCapacity(dps.Len())
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			histogramDp := histogramDps.AppendEmpty()
			dp.Attributes().CopyTo(histogramDp.Attributes())
			histogramDp.SetStartTimestamp(dp.StartTimestamp())
			histogramDp.SetTimestamp(dp.Timestamp())
			histogramDp.SetFlags(dp.Flags())
			histogramDp.SetCount(dp.Count())
			histogramDp.SetSum(dp.Sum())
			// A single bucket spanning (-Inf, +Inf): no explicit bounds, one count.
			histogramDp.BucketCounts().FromRaw([]uint64{dp.Count()})
		}
		return nil, nil
	}, nil
}
