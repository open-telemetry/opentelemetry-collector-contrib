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
