// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func (c *prometheusConverter) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, settings Settings, name string,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		labels, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, model.MetricNameLabel, name)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		sample := &prompb.Sample{
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
	return errs
}

func (c *prometheusConverter) addSumNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, _ pmetric.Metric, settings Settings, name string,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		lbls, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, true, c.labelNamer, model.MetricNameLabel, name)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		sample := &prompb.Sample{
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
		ts := c.addSample(sample, lbls)
		if ts != nil {
			exemplars := getPromExemplars[pmetric.NumberDataPoint](pt)
			ts.Exemplars = append(ts.Exemplars, exemplars...)
		}
	}
	return errs
}
