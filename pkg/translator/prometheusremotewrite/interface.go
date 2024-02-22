package prometheusremotewrite

import (
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// MetricsConverter defines the interface for converters from OTel metrics.
type MetricsConverter interface {
	// AddSample adds a sample and returns the corresponding TimeSeries.
	AddSample(*prompb.Sample, []prompb.Label) *prompb.TimeSeries
	// AddExemplars adds exemplars.
	AddExemplars(pmetric.HistogramDataPoint, []bucketBoundsData)
	// AddGaugeNumberDataPoints adds Gauge metric data points.
	AddGaugeNumberDataPoints(pmetric.NumberDataPointSlice, pcommon.Resource, Settings, string) error
	// AddSumNumberDataPoints adds Sum metric data points.
	AddSumNumberDataPoints(pmetric.NumberDataPointSlice, pcommon.Resource, pmetric.Metric, Settings, string) error
	// AddSummaryDataPoints adds summary metric data points, each converted to len(QuantileValues) + 2 samples.
	AddSummaryDataPoints(pmetric.SummaryDataPointSlice, pcommon.Resource, Settings, string) error
	// AddHistogramDataPoints adds histogram metric data points, each converted to 2 + min(len(ExplicitBounds), len(BucketCount)) + 1 samples.
	// It ignores extra buckets if len(ExplicitBounds) > len(BucketCounts).
	AddHistogramDataPoints(pmetric.HistogramDataPointSlice, pcommon.Resource, Settings, string) error
	// AddExponentialHistogramDataPoints adds exponential histogtram metric data points.
	AddExponentialHistogramDataPoints(pmetric.ExponentialHistogramDataPointSlice, pcommon.Resource, Settings, string) error
}
