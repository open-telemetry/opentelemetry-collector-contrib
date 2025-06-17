// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"
	"strconv"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// addResourceTargetInfoV2 converts the resource to the target info metric.
func (c *prometheusConverterV2) addResourceTargetInfoV2(resource pcommon.Resource, settings Settings, timestamp pcommon.Timestamp) {
	if settings.DisableTargetInfo || timestamp == 0 {
		return
	}

	attributes := resource.Attributes()
	identifyingAttrs := []string{
		string(conventions.ServiceNamespaceKey),
		string(conventions.ServiceNameKey),
		string(conventions.ServiceInstanceIDKey),
	}
	nonIdentifyingAttrsCount := attributes.Len()
	for _, a := range identifyingAttrs {
		_, haveAttr := attributes.Get(a)
		if haveAttr {
			nonIdentifyingAttrsCount--
		}
	}
	if nonIdentifyingAttrsCount == 0 {
		// If we only have job + instance, then target_info isn't useful, so don't add it.
		return
	}

	name := prometheustranslator.TargetInfoMetricName
	if len(settings.Namespace) > 0 {
		// TODO what to do with this in case of full utf-8 support?
		name = settings.Namespace + "_" + name
	}

	labels := createAttributes(resource, attributes, settings.ExternalLabels, identifyingAttrs, false, model.MetricNameLabel, name)
	haveIdentifier := false
	for _, l := range labels {
		if l.Name == model.JobLabel || l.Name == model.InstanceLabel {
			haveIdentifier = true
			break
		}
	}

	if !haveIdentifier {
		// We need at least one identifying label to generate target_info.
		return
	}

	sample := &writev2.Sample{
		Value: float64(1),
		// convert ns to ms
		Timestamp: convertTimeStamp(timestamp),
	}
	c.addSample(sample, labels, metadata{
		Type: writev2.Metadata_METRIC_TYPE_GAUGE,
		Help: "Target metadata",
	})
}

// addSampleWithLabels is a helper function to create and add a sample with labels
func (c *prometheusConverterV2) addSampleWithLabels(sampleValue float64, timestamp int64, noRecordedValue bool,
	baseName string, baseLabels []prompb.Label, labelName, labelValue string, metadata metadata,
) {
	sample := &writev2.Sample{
		Value:     sampleValue,
		Timestamp: timestamp,
	}
	if noRecordedValue {
		sample.Value = math.Float64frombits(value.StaleNaN)
	}
	if labelName != "" && labelValue != "" {
		c.addSample(sample, createLabels(baseName, baseLabels, labelName, labelValue), metadata)
	} else {
		c.addSample(sample, createLabels(baseName, baseLabels), metadata)
	}
}

func (c *prometheusConverterV2) addSummaryDataPoints(dataPoints pmetric.SummaryDataPointSlice, resource pcommon.Resource,
	settings Settings, baseName string, metadata metadata,
) {
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nil, false)
		noRecordedValue := pt.Flags().NoRecordedValue()

		// Add sum and count samples
		c.addSampleWithLabels(pt.Sum(), timestamp, noRecordedValue, baseName+sumStr, baseLabels, "", "", metadata)
		c.addSampleWithLabels(float64(pt.Count()), timestamp, noRecordedValue, baseName+countStr, baseLabels, "", "", metadata)

		// Process quantiles
		for i := 0; i < pt.QuantileValues().Len(); i++ {
			qt := pt.QuantileValues().At(i)
			percentileStr := strconv.FormatFloat(qt.Quantile(), 'f', -1, 64)
			c.addSampleWithLabels(qt.Value(), timestamp, noRecordedValue, baseName, baseLabels, quantileStr, percentileStr, metadata)
		}
	}
}

func (c *prometheusConverterV2) addHistogramDataPoints(dataPoints pmetric.HistogramDataPointSlice,
	resource pcommon.Resource, settings Settings, baseName string, metadata metadata,
) {
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nil, false)

		// If the sum is unset, it indicates the _sum metric point should be
		// omitted
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
			c.addSample(sum, sumlabels, metadata)
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
		c.addSample(count, countlabels, metadata)

		// cumulative count for conversion to cumulative histogram
		var cumulativeCount uint64

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
			c.addSample(bucket, labels, metadata)
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
		c.addSample(infBucket, infLabels, metadata)

		// TODO implement exemplars support
	}
}
