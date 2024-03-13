// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

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

// convertMetrics converts pmetric.Metrics to another metrics format via the converter.
func convertMetrics(md pmetric.Metrics, settings Settings, converter MetricsConverter) (errs error) {
	resourceMetricsSlice := md.ResourceMetrics()
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		resourceMetrics := resourceMetricsSlice.At(i)
		resource := resourceMetrics.Resource()
		scopeMetricsSlice := resourceMetrics.ScopeMetrics()
		// keep track of the most recent timestamp in the ResourceMetrics for
		// use with the "target" info metric
		var mostRecentTimestamp pcommon.Timestamp
		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			metricSlice := scopeMetricsSlice.At(j).Metrics()

			// TODO: decide if instrumentation library information should be exported as labels
			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				mostRecentTimestamp = maxTimestamp(mostRecentTimestamp, mostRecentTimestampInMetric(metric))

				if !isValidAggregationTemporality(metric) {
					errs = multierr.Append(errs, fmt.Errorf("invalid temporality and type combination for metric %q", metric.Name()))
					continue
				}

				promName := prometheustranslator.BuildCompliantName(metric, settings.Namespace, settings.AddMetricSuffixes)

				// handle individual metrics based on type
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPoints := metric.Gauge().DataPoints()
					if dataPoints.Len() == 0 {
						errs = multierr.Append(errs, fmt.Errorf("empty data points. %s is dropped", metric.Name()))
						break
					}
					errs = multierr.Append(errs, converter.AddGaugeNumberDataPoints(dataPoints, resource, settings, promName))
				case pmetric.MetricTypeSum:
					dataPoints := metric.Sum().DataPoints()
					if dataPoints.Len() == 0 {
						errs = multierr.Append(errs, fmt.Errorf("empty data points. %s is dropped", metric.Name()))
						break
					}
					errs = multierr.Append(errs, converter.AddSumNumberDataPoints(dataPoints, resource, metric, settings, promName))
				case pmetric.MetricTypeHistogram:
					dataPoints := metric.Histogram().DataPoints()
					if dataPoints.Len() == 0 {
						errs = multierr.Append(errs, fmt.Errorf("empty data points. %s is dropped", metric.Name()))
						break
					}
					errs = multierr.Append(errs, converter.AddHistogramDataPoints(dataPoints, resource, settings, promName))
				case pmetric.MetricTypeExponentialHistogram:
					dataPoints := metric.ExponentialHistogram().DataPoints()
					if dataPoints.Len() == 0 {
						errs = multierr.Append(errs, fmt.Errorf("empty data points. %s is dropped", metric.Name()))
						break
					}
					errs = multierr.Append(errs, converter.AddExponentialHistogramDataPoints(
						dataPoints,
						resource,
						settings,
						promName,
					))
				case pmetric.MetricTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					if dataPoints.Len() == 0 {
						errs = multierr.Append(errs, fmt.Errorf("empty data points. %s is dropped", metric.Name()))
						break
					}
					errs = multierr.Append(errs, converter.AddSummaryDataPoints(dataPoints, resource, settings, promName))
				default:
					errs = multierr.Append(errs, errors.New("unsupported metric type"))
				}
			}
		}
		addResourceTargetInfo(resource, settings, mostRecentTimestamp, converter)
	}

	return
}
