package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertExponentialHistToBucketedHistArguments struct {
	ExplicitBounds []float64
}

func newConvertExponentialHistToBucketedHistFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_exponential_hist_to_bucketed_hist", &convertExponentialHistToBucketedHistArguments{}, createConvertExponentialHistToBucketedHistFunction)
}

func createConvertExponentialHistToBucketedHistFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertExponentialHistToBucketedHistArguments)

	if !ok {
		return nil, fmt.Errorf("ConvertExponentialHistToBucketedHistFactory args must be of type *ConvertExponentialHistToBucketedHistArguments")
	}

	return convertExponentialHistToBucketedHist(args.ExplicitBounds)
}

// convertExponentialHistToBucketedHist converts an exponential histogram to a bucketed histogram
func convertExponentialHistToBucketedHist(explicitBounds []float64) (ottl.ExprFunc[ottlmetric.TransformContext], error) {

	if len(explicitBounds) == 0 {
		return nil, fmt.Errorf("explicit bounds must cannot be empty: %v", explicitBounds)
	}

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		// only execute on exponential histograms
		if metric.Type() != pmetric.MetricTypeExponentialHistogram {
			return nil, nil
		}

		// expHist := metric.ExponentialHistogram()
		bucketedHist := pmetric.NewHistogram()
		dps := metric.ExponentialHistogram().DataPoints()
		bucketedHist.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())

		for i := 0; i < dps.Len(); i++ {
			expDataPoint := dps.At(i)
			bucketCounts := calculateBucketCounts(expDataPoint, explicitBounds)
			bucketHistDatapoint := bucketedHist.DataPoints().AppendEmpty()
			bucketHistDatapoint.SetStartTimestamp(expDataPoint.StartTimestamp())
			bucketHistDatapoint.SetTimestamp(expDataPoint.Timestamp())
			bucketHistDatapoint.SetCount(expDataPoint.Count())
			bucketHistDatapoint.SetSum(expDataPoint.Sum())
			bucketHistDatapoint.SetMin(expDataPoint.Min())
			bucketHistDatapoint.SetMax(expDataPoint.Max())
			bucketHistDatapoint.ExplicitBounds().FromRaw(explicitBounds)
			bucketHistDatapoint.BucketCounts().FromRaw(bucketCounts)
			expDataPoint.Attributes().CopyTo(bucketHistDatapoint.Attributes())
		}

		// create new metric and override metric
		newMetric := pmetric.NewMetric()
		newMetric.SetName(metric.Name())
		newMetric.SetDescription(metric.Description())
		newMetric.SetUnit(metric.Unit())
		bucketedHist.CopyTo(newMetric.SetEmptyHistogram())
		newMetric.CopyTo(metric)

		return nil, nil
	}, nil
}

func calculateBucketCounts(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64) []uint64 {
	bucketCounts := make([]uint64, len(boundaries)+1) // +1 for the overflow bucket
	scale := dp.Scale()
	cv := 1 << scale // 2^scale
	currentValue := float64(cv)

	// Positive buckets
	for i := 0; i < int(dp.Positive().BucketCounts().Len()); i++ {
		count := dp.Positive().BucketCounts().At(i)
		// lowerBound := currentValue
		upperBound := currentValue * 2

		// Find the bucket range and add the count
		for j, boundary := range boundaries {
			if upperBound <= boundary {
				bucketCounts[j] += count
				break
			}
			if j == len(boundaries)-1 {
				bucketCounts[j+1] += count // Overflow bucket
			}
		}

		// Update currentValue to the next power of 2
		currentValue = upperBound
	}

	return bucketCounts
}
