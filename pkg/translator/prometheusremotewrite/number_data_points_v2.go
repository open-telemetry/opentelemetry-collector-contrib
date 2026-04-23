// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"encoding/hex"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func (c *prometheusConverterV2) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, settings Settings, name string, metadata metadata,
) error {
	var errs error
	symbolize := func(s string) uint32 { return c.symbolTable.Symbolize(s) }
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)

		labels, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, settings.DisableScopeInfo, model.MetricNameLabel, name)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
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
		ts := c.addSample(sample, labels, metadata)
		ts.Exemplars = append(ts.Exemplars, getPromExemplarsV2(pt, symbolize)...)
	}
	return errs
}

func (c *prometheusConverterV2) addSumNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, _ pmetric.Metric, settings Settings, name string, metadata metadata,
) error {
	var errs error
	symbolize := func(s string) uint32 { return c.symbolTable.Symbolize(s) }
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		lbls, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, settings.DisableScopeInfo, model.MetricNameLabel, name)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

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
		ts := c.addSample(sample, lbls, metadata)
		ts.Exemplars = append(ts.Exemplars, getPromExemplarsV2(pt, symbolize)...)
	}
	return errs
}

// getPromExemplarsV2 returns a slice of writev2.Exemplar from pdata exemplars.
func getPromExemplarsV2[T exemplarType](pt T, symbolize func(string) uint32) []writev2.Exemplar {
	promExemplars := make([]writev2.Exemplar, 0, pt.Exemplars().Len())
	for i := 0; i < pt.Exemplars().Len(); i++ {
		exemplar := pt.Exemplars().At(i)
		exemplarRunes := 0

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

		var labels []prompb.Label

		if traceID := exemplar.TraceID(); !traceID.IsEmpty() {
			val := hex.EncodeToString(traceID[:])
			exemplarRunes += utf8.RuneCountInString(otlptranslator.ExemplarTraceIDKey) + utf8.RuneCountInString(val)
			labels = append(labels, prompb.Label{
				Name:  otlptranslator.ExemplarTraceIDKey,
				Value: val,
			})
		}
		if spanID := exemplar.SpanID(); !spanID.IsEmpty() {
			val := hex.EncodeToString(spanID[:])
			exemplarRunes += utf8.RuneCountInString(otlptranslator.ExemplarSpanIDKey) + utf8.RuneCountInString(val)
			labels = append(labels, prompb.Label{
				Name:  otlptranslator.ExemplarSpanIDKey,
				Value: val,
			})
		}

		attrs := exemplar.FilteredAttributes()
		var labelsFromAttributes []prompb.Label
		for key, value := range attrs.All() {
			if key == otlptranslator.ExemplarTraceIDKey && !exemplar.TraceID().IsEmpty() {
				continue
			}
			if key == otlptranslator.ExemplarSpanIDKey && !exemplar.SpanID().IsEmpty() {
				continue
			}
			val := value.AsString()
			exemplarRunes += utf8.RuneCountInString(key) + utf8.RuneCountInString(val)
			labelsFromAttributes = append(labelsFromAttributes, prompb.Label{
				Name:  key,
				Value: val,
			})
		}

		if exemplarRunes <= maxExemplarRunes {
			labels = append(labels, labelsFromAttributes...)
		}

		sort.Sort(ByLabelName(labels))

		// Symbolize labels and store references
		if len(labels) > 0 {
			buf := make([]uint32, 0, len(labels)*2)
			for _, l := range labels {
				buf = append(buf, symbolize(l.Name), symbolize(l.Value))
			}
			promExemplar.LabelsRefs = buf
		}

		promExemplars = append(promExemplars, promExemplar)
	}

	return promExemplars
}
