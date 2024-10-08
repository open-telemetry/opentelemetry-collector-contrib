// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"github.com/prometheus/prometheus/model/value"
	"math"
	"testing"
	"time"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestPrometheusConverterV2_addGaugeNumberDataPoints(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[uint64]*writev2.TimeSeries
	}{
		{
			name: "gauge",
			metric: func() pmetric.Metric {
				return getIntGaugeMetric(
					"test",
					pcommon.NewMap(),
					1, ts,
				)
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := labels.Labels{
					labels.Label{
						Name:  labels.MetricName,
						Value: "test",
					},
				}
				return map[uint64]*writev2.TimeSeries{
					labels.Hash(): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1},
						},
					},
				}
			},
		},
		{
			name: "gauge with staleNaN",
			metric: func() pmetric.Metric {
				return getIntGaugeMetric(
					"staleNaN",
					pcommon.NewMap(),
					1, ts,
				)
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := labels.Labels{
					labels.Label{
						Name:  labels.MetricName,
						Value: "staleNaN",
					},
				}
				return map[uint64]*writev2.TimeSeries{
					labels.Hash(): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: math.Float64frombits(value.StaleNaN)},
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()
			converter := newPrometheusConverterV2()
			converter.addGaugeNumberDataPoints(metric.Gauge().DataPoints(), metric.Name())
			w := tt.want()
			// assert.Equal(t, w, converter.unique)
			for k, v := range w {
				assert.Equal(t, *v, *converter.unique[k])
			}

		})
	}
}

// Right now we are not handling duplicates, the second one will just overwrite the first one as this test case shows
// TODO handle duplicates
func TestPrometheusConverterV2_addGaugeNumberDataPointsDuplicate(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	metric1 := getIntGaugeMetric(
		"test",
		pcommon.NewMap(),
		1, ts,
	)
	metric2 := getIntGaugeMetric(
		"test",
		pcommon.NewMap(),
		2, ts,
	)
	want := func() map[uint64]*writev2.TimeSeries {
		labels := labels.Labels{
			labels.Label{
				Name:  labels.MetricName,
				Value: "test",
			},
		}
		return map[uint64]*writev2.TimeSeries{
			labels.Hash(): {
				LabelsRefs: []uint32{1, 2},
				Samples: []writev2.Sample{
					{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 2},
				},
			},
		}
	}

	t.Run("gauge duplicate", func(t *testing.T) {
		converter := newPrometheusConverterV2()
		converter.addGaugeNumberDataPoints(metric1.Gauge().DataPoints(), metric1.Name())
		converter.addGaugeNumberDataPoints(metric2.Gauge().DataPoints(), metric2.Name())

		assert.Equal(t, want(), converter.unique)

	})

}
