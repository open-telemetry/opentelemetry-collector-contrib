// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// decomposeHistograms decomposes any histograms present in the metric data into individual Sums and Gauges
// This is a noop if no Histograms are present, but otherwise makes a copy of the whole structure
// This exists because Sumo doesn't support OTLP histograms yet, and has the same semantics as the conversion to Prometheus format in prometheus_formatter.go
func decomposeHistograms(md pmetric.Metrics) pmetric.Metrics {
	// short circuit and do nothing if no Histograms are present
	foundHistogram := false
outer:
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				foundHistogram = scopeMetric.Metrics().At(k).Type() == pmetric.MetricTypeHistogram
				if foundHistogram {
					break outer
				}
			}
		}
	}
	if !foundHistogram {
		return md
	}

	decomposed := pmetric.NewMetrics()
	md.CopyTo(decomposed)

	for i := 0; i < decomposed.ResourceMetrics().Len(); i++ {
		resourceMetric := decomposed.ResourceMetrics().At(i)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			metrics := resourceMetric.ScopeMetrics().At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				if metric.Type() == pmetric.MetricTypeHistogram {
					decomposedHistogram := decomposeHistogram(metric)
					decomposedHistogram.MoveAndAppendTo(metrics)
				}
			}
			metrics.RemoveIf(func(m pmetric.Metric) bool { return m.Type() == pmetric.MetricTypeHistogram })
		}
	}

	return decomposed
}

// decomposeHistogram decomposes a single Histogram metric into individual metrics for count, bucket and sum
// non-Histograms give an empty slice as output
func decomposeHistogram(metric pmetric.Metric) pmetric.MetricSlice {
	output := pmetric.NewMetricSlice()
	if metric.Type() != pmetric.MetricTypeHistogram {
		return output
	}

	getHistogramSumMetric(metric).MoveTo(output.AppendEmpty())
	getHistogramCountMetric(metric).MoveTo(output.AppendEmpty())
	getHistogramBucketsMetric(metric).MoveTo(output.AppendEmpty())

	return output
}

func getHistogramBucketsMetric(metric pmetric.Metric) pmetric.Metric {
	histogram := metric.Histogram()

	bucketsMetric := pmetric.NewMetric()
	bucketsMetric.SetName(metric.Name() + "_bucket")
	bucketsMetric.SetDescription(metric.Description())
	bucketsMetric.SetUnit(metric.Unit())
	bucketsMetric.SetEmptyGauge()
	bucketsDatapoints := bucketsMetric.Gauge().DataPoints()

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		histogramDataPoint := histogram.DataPoints().At(i)
		histogramBounds := histogramDataPoint.ExplicitBounds()
		var cumulative uint64

		for j := 0; j < histogramBounds.Len(); j++ {
			bucketDataPoint := bucketsDatapoints.AppendEmpty()
			bound := histogramBounds.At(j)
			histogramDataPoint.Attributes().CopyTo(bucketDataPoint.Attributes())

			if math.IsInf(bound, 1) {
				bucketDataPoint.Attributes().PutStr(prometheusLeTag, prometheusInfValue)
			} else {
				bucketDataPoint.Attributes().PutDouble(prometheusLeTag, bound)
			}

			bucketDataPoint.SetStartTimestamp(histogramDataPoint.StartTimestamp())
			bucketDataPoint.SetTimestamp(histogramDataPoint.Timestamp())
			cumulative += histogramDataPoint.BucketCounts().At(j)
			bucketDataPoint.SetIntValue(int64(cumulative))
		}

		// need to add one more bucket at +Inf
		bucketDataPoint := bucketsDatapoints.AppendEmpty()
		histogramDataPoint.Attributes().CopyTo(bucketDataPoint.Attributes())
		bucketDataPoint.Attributes().PutStr(prometheusLeTag, prometheusInfValue)
		bucketDataPoint.SetStartTimestamp(histogramDataPoint.StartTimestamp())
		bucketDataPoint.SetTimestamp(histogramDataPoint.Timestamp())
		cumulative += histogramDataPoint.BucketCounts().At(histogramDataPoint.ExplicitBounds().Len())
		bucketDataPoint.SetIntValue(int64(cumulative))
	}
	return bucketsMetric
}

func getHistogramSumMetric(metric pmetric.Metric) pmetric.Metric {
	histogram := metric.Histogram()

	sumMetric := pmetric.NewMetric()
	sumMetric.SetName(metric.Name() + "_sum")
	sumMetric.SetDescription(metric.Description())
	sumMetric.SetUnit(metric.Unit())
	sumMetric.SetEmptyGauge()
	sumDataPoints := sumMetric.Gauge().DataPoints()

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		histogramDataPoint := histogram.DataPoints().At(i)
		sumDataPoint := sumDataPoints.AppendEmpty()
		histogramDataPoint.Attributes().CopyTo(sumDataPoint.Attributes())
		sumDataPoint.SetStartTimestamp(histogramDataPoint.StartTimestamp())
		sumDataPoint.SetTimestamp(histogramDataPoint.Timestamp())
		sumDataPoint.SetDoubleValue(histogramDataPoint.Sum())
	}
	return sumMetric
}

func getHistogramCountMetric(metric pmetric.Metric) pmetric.Metric {
	histogram := metric.Histogram()

	countMetric := pmetric.NewMetric()
	countMetric.SetName(metric.Name() + "_count")
	countMetric.SetDescription(metric.Description())
	countMetric.SetUnit(metric.Unit())
	countMetric.SetEmptyGauge()
	countDataPoints := countMetric.Gauge().DataPoints()

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		histogramDataPoint := histogram.DataPoints().At(i)
		countDataPoint := countDataPoints.AppendEmpty()
		histogramDataPoint.Attributes().CopyTo(countDataPoint.Attributes())
		countDataPoint.SetStartTimestamp(histogramDataPoint.StartTimestamp())
		countDataPoint.SetTimestamp(histogramDataPoint.Timestamp())
		countDataPoint.SetIntValue(int64(histogramDataPoint.Count()))
	}
	return countMetric
}

// decomposeSummaries decomposes any summaries present in the metric data into individual Gauges and Sums
// This is a noop if no Summaries are present, but otherwise makes a copy of the whole structure
// This exists because Sumo doesn't support OTLP summaries with proper quantile dimensions
func decomposeSummaries(md pmetric.Metrics) pmetric.Metrics {
	// short circuit and do nothing if no Summaries are present
	foundSummary := false
outer:
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				foundSummary = scopeMetric.Metrics().At(k).Type() == pmetric.MetricTypeSummary
				if foundSummary {
					break outer
				}
			}
		}
	}
	if !foundSummary {
		return md
	}

	decomposed := pmetric.NewMetrics()
	md.CopyTo(decomposed)

	for i := 0; i < decomposed.ResourceMetrics().Len(); i++ {
		resourceMetric := decomposed.ResourceMetrics().At(i)
		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			metrics := resourceMetric.ScopeMetrics().At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				if metric.Type() == pmetric.MetricTypeSummary {
					decomposedSummary := decomposeSummary(metric)
					decomposedSummary.MoveAndAppendTo(metrics)
				}
			}
			metrics.RemoveIf(func(m pmetric.Metric) bool { return m.Type() == pmetric.MetricTypeSummary })
		}
	}

	return decomposed
}

// decomposeSummary decomposes a single Summary metric into individual metrics for quantiles, count, and sum
// non-Summaries give an empty slice as output
func decomposeSummary(metric pmetric.Metric) pmetric.MetricSlice {
	output := pmetric.NewMetricSlice()
	if metric.Type() != pmetric.MetricTypeSummary {
		return output
	}

	getSummaryQuantilesMetric(metric).MoveAndAppendTo(output)
	getSummaryCountMetric(metric).MoveTo(output.AppendEmpty())
	getSummarySumMetric(metric).MoveTo(output.AppendEmpty())

	return output
}

// getSummaryQuantilesMetric extracts quantile values from a summary as individual gauge datapoints
// Each quantile becomes a separate datapoint with a "quantile" attribute
func getSummaryQuantilesMetric(metric pmetric.Metric) pmetric.MetricSlice {
	summary := metric.Summary()
	output := pmetric.NewMetricSlice()

	// Create one gauge metric for all quantiles (multiple datapoints with quantile attribute)
	quantilesMetric := pmetric.NewMetric()
	quantilesMetric.SetName(metric.Name())
	quantilesMetric.SetDescription(metric.Description())
	quantilesMetric.SetUnit(metric.Unit())
	quantilesMetric.SetEmptyGauge()
	quantilesDataPoints := quantilesMetric.Gauge().DataPoints()

	for i := 0; i < summary.DataPoints().Len(); i++ {
		summaryDataPoint := summary.DataPoints().At(i)
		quantileValues := summaryDataPoint.QuantileValues()

		for j := 0; j < quantileValues.Len(); j++ {
			quantileValue := quantileValues.At(j)
			gaugeDataPoint := quantilesDataPoints.AppendEmpty()

			// Copy original attributes
			summaryDataPoint.Attributes().CopyTo(gaugeDataPoint.Attributes())

			// Add quantile as an attribute
			gaugeDataPoint.Attributes().PutDouble("quantile", quantileValue.Quantile())

			// Set timestamps and value
			gaugeDataPoint.SetStartTimestamp(summaryDataPoint.StartTimestamp())
			gaugeDataPoint.SetTimestamp(summaryDataPoint.Timestamp())
			gaugeDataPoint.SetDoubleValue(quantileValue.Value())
		}
	}

	quantilesMetric.MoveTo(output.AppendEmpty())
	return output
}

func getSummaryCountMetric(metric pmetric.Metric) pmetric.Metric {
	summary := metric.Summary()

	countMetric := pmetric.NewMetric()
	countMetric.SetName(metric.Name() + "_count")
	countMetric.SetDescription(metric.Description())
	countMetric.SetUnit("1") // count is unitless
	countMetric.SetEmptySum()

	// Set sum properties for counter-like behavior
	sum := countMetric.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	countDataPoints := sum.DataPoints()

	for i := 0; i < summary.DataPoints().Len(); i++ {
		summaryDataPoint := summary.DataPoints().At(i)
		countDataPoint := countDataPoints.AppendEmpty()
		summaryDataPoint.Attributes().CopyTo(countDataPoint.Attributes())
		countDataPoint.SetStartTimestamp(summaryDataPoint.StartTimestamp())
		countDataPoint.SetTimestamp(summaryDataPoint.Timestamp())
		countDataPoint.SetIntValue(int64(summaryDataPoint.Count()))
	}
	return countMetric
}

func getSummarySumMetric(metric pmetric.Metric) pmetric.Metric {
	summary := metric.Summary()

	sumMetric := pmetric.NewMetric()
	sumMetric.SetName(metric.Name() + "_sum")
	sumMetric.SetDescription(metric.Description())
	sumMetric.SetUnit(metric.Unit())
	sumMetric.SetEmptySum()

	// Set sum properties for counter-like behavior
	sum := sumMetric.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	sumDataPoints := sum.DataPoints()

	for i := 0; i < summary.DataPoints().Len(); i++ {
		summaryDataPoint := summary.DataPoints().At(i)
		sumDataPoint := sumDataPoints.AppendEmpty()
		summaryDataPoint.Attributes().CopyTo(sumDataPoint.Attributes())
		sumDataPoint.SetStartTimestamp(summaryDataPoint.StartTimestamp())
		sumDataPoint.SetTimestamp(summaryDataPoint.Timestamp())
		sumDataPoint.SetDoubleValue(summaryDataPoint.Sum())
	}
	return sumMetric
}
