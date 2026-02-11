// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func (c *prometheusConverterV2) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, settings Settings, name string, metadata metadata,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)

		labels, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, model.MetricNameLabel, name)
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
		c.addSample(sample, labels, metadata)
	}
	return errs
}

func (c *prometheusConverterV2) addSumNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, _ pmetric.Metric, settings Settings, name string, metadata metadata,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		lbls, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, model.MetricNameLabel, name)
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
		// TODO: properly add exemplars to the TimeSeries
		c.addSample(sample, lbls, metadata)
	}
	return errs
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
