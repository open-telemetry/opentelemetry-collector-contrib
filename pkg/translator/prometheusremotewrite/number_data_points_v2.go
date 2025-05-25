// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"
	"strconv"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (c *prometheusConverterV2) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, settings Settings, name string,
) {
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)

		labels := createAttributes(
			resource,
			pt.Attributes(),
			settings.ExternalLabels,
			nil,
			true,
			model.MetricNameLabel,
			name,
		)

		sample := &writev2.Sample{
			// convert ns to ms
			Timestamp: convertTimeStamp(pt.Timestamp()),
		}
		switch pt.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			sample.Value = float64(pt.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			sample.Value = pt.DoubleValue()
		}
		if pt.Flags().NoRecordedValue() {
			sample.Value = math.Float64frombits(value.StaleNaN)
		}
		c.addSample(sample, labels)
	}
}

func (c *prometheusConverterV2) addSumNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, _ pmetric.Metric, settings Settings, name string,
) {
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		lbls := createAttributes(
			resource,
			pt.Attributes(),
			settings.ExternalLabels,
			nil,
			true,
			model.MetricNameLabel,
			name,
		)

		sample := &writev2.Sample{
			// convert ns to ms
			Timestamp: convertTimeStamp(pt.Timestamp()),
		}
		switch pt.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			sample.Value = float64(pt.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			sample.Value = pt.DoubleValue()
		}
		if pt.Flags().NoRecordedValue() {
			sample.Value = math.Float64frombits(value.StaleNaN)
		}
		// TODO: properly add exemplars to the TimeSeries
		c.addSample(sample, lbls)
	}
}

// addHistogramDataPoints converts histogram metrics from OTLP to Prometheus remote write format
func (c *prometheusConverterV2) addHistogramDataPoints(dataPoints pmetric.HistogramDataPointSlice,
	resource pcommon.Resource, settings Settings, baseName string,
) {
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nil, false)

		// If the sum is unset, it indicates the _sum metric point should be omitted
		if pt.HasSum() {
			// treat sum as a sample in an individual TimeSeries
			sum := &writev2.Sample{
				Value:     pt.Sum(),
				Timestamp: timestamp,
			}
			if pt.Flags().NoRecordedValue() {
				sum.Value = math.Float64frombits(value.StaleNaN)
			}

			sumlabels := createLabels(baseName+sumStr, baseLabels)
			c.addSample(sum, sumlabels)
		}

		// treat count as a sample in an individual TimeSeries
		count := &writev2.Sample{
			Value:     float64(pt.Count()),
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			count.Value = math.Float64frombits(value.StaleNaN)
		}

		countlabels := createLabels(baseName+countStr, baseLabels)
		c.addSample(count, countlabels)

		// cumulative count for conversion to cumulative histogram
		var cumulativeCount uint64

		var bucketBounds []bucketBoundsDataV2

		// process each bound, based on histograms proto definition, # of buckets = # of explicit bounds + 1
		for i := 0; i < pt.ExplicitBounds().Len() && i < pt.BucketCounts().Len(); i++ {
			bound := pt.ExplicitBounds().At(i)
			cumulativeCount += pt.BucketCounts().At(i)
			bucket := &writev2.Sample{
				Value:     float64(cumulativeCount),
				Timestamp: timestamp,
			}
			if pt.Flags().NoRecordedValue() {
				bucket.Value = math.Float64frombits(value.StaleNaN)
			}
			boundStr := strconv.FormatFloat(bound, 'f', -1, 64)
			labels := createLabels(baseName+bucketStr, baseLabels, leStr, boundStr)

			// Add the sample and store the signature for later exemplar lookup
			c.addSample(bucket, labels)
			signature := timeSeriesSignature(labels)

			bucketBounds = append(bucketBounds, bucketBoundsDataV2{
				signature: signature,
				bound:     bound,
			})
		}

		// add le=+Inf bucket
		infBucket := &writev2.Sample{
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			infBucket.Value = math.Float64frombits(value.StaleNaN)
		} else {
			infBucket.Value = float64(pt.Count())
		}
		infLabels := createLabels(baseName+bucketStr, baseLabels, leStr, pInfStr)

		// Add the inf bucket sample and store its signature
		c.addSample(infBucket, infLabels)
		signature := timeSeriesSignature(infLabels)

		bucketBounds = append(bucketBounds, bucketBoundsDataV2{
			signature: signature,
			bound:     math.Inf(1),
		})

		c.addExemplars(pt, bucketBounds)
	}
}

// getPromExemplarsV2 returns a slice of writev2.Exemplar from pdata exemplars.
func getPromExemplarsV2[T exemplarType](pt T) []writev2.Exemplar {
	promExemplars := make([]writev2.Exemplar, 0, pt.Exemplars().Len())
	for i := 0; i < pt.Exemplars().Len(); i++ {
		exemplar := pt.Exemplars().At(i)

		var promExemplar writev2.Exemplar

		switch exemplar.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			promExemplar = writev2.Exemplar{
				Value:     float64(exemplar.IntValue()),
				Timestamp: timestamp.FromTime(exemplar.Timestamp().AsTime()),
			}
		case pmetric.ExemplarValueTypeDouble:
			promExemplar = writev2.Exemplar{
				Value:     exemplar.DoubleValue(),
				Timestamp: timestamp.FromTime(exemplar.Timestamp().AsTime()),
			}
		}
		// TODO append labels to promExemplar.Labels

		promExemplars = append(promExemplars, promExemplar)
	}

	return promExemplars
}
