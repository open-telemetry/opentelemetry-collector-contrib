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
}

func newConvertSummaryToHistogramFactory() ottl.Factory[*ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_summary_to_histogram", &convertSummaryToHistogramArguments{}, createConvertSummaryToHistogramFunction)
}

func createConvertSummaryToHistogramFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertSummaryToHistogramArguments)
	if !ok {
		return nil, errors.New("convertSummaryToHistogramFactory args must be of type *convertSummaryToHistogramArguments")
	}

	return convertSummaryToHistogram(args.StringAggTemp)
}

func convertSummaryToHistogram(stringAggTemp string) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
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

		summaryDps := metric.Summary().DataPoints()

		// SetEmptyHistogram replaces the metric's data but the Summary data points captured above stay readable.
		histogram := metric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(aggTemp)

		histogramDps := histogram.DataPoints()
		histogramDps.EnsureCapacity(summaryDps.Len())
		for i := 0; i < summaryDps.Len(); i++ {
			dp := summaryDps.At(i)
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
