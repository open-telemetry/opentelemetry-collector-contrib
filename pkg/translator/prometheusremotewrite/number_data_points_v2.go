// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (c *prometheusConverterV2) addGaugeNumberDataPoints(dataPoints pmetric.NumberDataPointSlice,
	resource pcommon.Resource, settings Settings, name string) {
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
