// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
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
			name: "int_gauge",
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
			name: "double_gauge",
			metric: func() pmetric.Metric {
				return getDoubleGaugeMetric(
					"test",
					pcommon.NewMap(),
					1.5, ts,
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
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.5},
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
			settings := Settings{
				Namespace:         "",
				ExternalLabels:    nil,
				DisableTargetInfo: false,
				SendMetadata:      false,
			}
			converter := newPrometheusConverterV2()
			converter.addGaugeNumberDataPoints(metric.Gauge().DataPoints(), pcommon.NewResource(), settings, metric.Name())
			w := tt.want()

			diff := cmp.Diff(w, converter.unique, cmpopts.EquateNaNs())
			assert.Empty(t, diff)
		})
	}
}

// Right now we are not handling duplicates, the second one will just overwrite the first one as this test case shows
// In follow-up PRs we plan to start handling conflicts and this test will be updated to reflect the new behavior.
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

	settings := Settings{
		Namespace:         "",
		ExternalLabels:    nil,
		DisableTargetInfo: false,
		SendMetadata:      false,
	}

	converter := newPrometheusConverterV2()
	converter.addGaugeNumberDataPoints(metric1.Gauge().DataPoints(), pcommon.NewResource(), settings, metric1.Name())
	converter.addGaugeNumberDataPoints(metric2.Gauge().DataPoints(), pcommon.NewResource(), settings, metric2.Name())

	assert.Equal(t, want(), converter.unique)
}
