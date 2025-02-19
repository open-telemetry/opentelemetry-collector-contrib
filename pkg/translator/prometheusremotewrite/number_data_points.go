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
)

func (c *prometheusConverter) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, settings Settings, name string,
) {
	for x := range dataPoints.Len() {
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
}

func (c *prometheusConverter) addSumNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, metric pmetric.Metric, settings Settings, name string,
) {
	for x := range dataPoints.Len() {
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

		// add created time series if needed
		if settings.ExportCreatedMetric && metric.Sum().IsMonotonic() {
			startTimestamp := pt.StartTimestamp()
			if startTimestamp == 0 {
				return
			}

			createdLabels := make([]prompb.Label, len(lbls))
			copy(createdLabels, lbls)
			for i, l := range createdLabels {
				if l.Name == model.MetricNameLabel {
					createdLabels[i].Value = name + createdSuffix
					break
				}
			}
			c.addTimeSeriesIfNeeded(createdLabels, startTimestamp, pt.Timestamp())
		}
	}
}
