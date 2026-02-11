// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"
	"strconv"

	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
)

// addResourceTargetInfoV2 converts the resource to the target info metric.
func (c *prometheusConverterV2) addResourceTargetInfoV2(resource pcommon.Resource, settings Settings, timestamp pcommon.Timestamp) error {
	if settings.DisableTargetInfo || timestamp == 0 {
		return nil
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
		return nil
	}

	name := otlptranslator.TargetInfoMetricName
	if settings.Namespace != "" {
		// TODO what to do with this in case of full utf-8 support?
		name = settings.Namespace + "_" + name
	}

	labels, err := createAttributes(resource, attributes, pcommon.NewInstrumentationScope(), settings.ExternalLabels, identifyingAttrs, false, c.labelNamer, model.MetricNameLabel, name)
	if err != nil {
		return err
	}
	haveIdentifier := false
	for _, l := range labels {
		if l.Name == model.JobLabel || l.Name == model.InstanceLabel {
			haveIdentifier = true
			break
		}
	}

	if !haveIdentifier {
		// We need at least one identifying label to generate target_info.
		return nil
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
	return nil
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
	scope pcommon.InstrumentationScope, settings Settings, baseName string, metadata metadata,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, false, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
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
	return errs
}

func (c *prometheusConverterV2) addHistogramDataPoints(dataPoints pmetric.HistogramDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, settings Settings, baseName string, metadata metadata,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, false, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		noRecordedValue := pt.Flags().NoRecordedValue()

		// If the sum is unset, it indicates the _sum metric point should be
		// omitted
		if pt.HasSum() {
			c.addSampleWithLabels(pt.Sum(), timestamp, noRecordedValue, baseName+sumStr, baseLabels, "", "", metadata)
		}

		// treat count as a sample in an individual TimeSeries
		c.addSampleWithLabels(float64(pt.Count()), timestamp, noRecordedValue, baseName+countStr, baseLabels, "", "", metadata)

		// cumulative count for conversion to cumulative histogram
		var cumulativeCount uint64

		// process each bound, based on histograms proto definition, # of buckets = # of explicit bounds + 1
		for i := 0; i < pt.ExplicitBounds().Len() && i < pt.BucketCounts().Len(); i++ {
			bound := pt.ExplicitBounds().At(i)
			cumulativeCount += pt.BucketCounts().At(i)
			boundStr := strconv.FormatFloat(bound, 'f', -1, 64)
			c.addSampleWithLabels(float64(cumulativeCount), timestamp, noRecordedValue, baseName+bucketStr, baseLabels, leStr, boundStr, metadata)
		}
		// add le=+Inf bucket
		c.addSampleWithLabels(float64(pt.Count()), timestamp, noRecordedValue, baseName+bucketStr, baseLabels, leStr, pInfStr, metadata)

		// TODO implement exemplars support
	}
	return errs
}
