// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
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
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 3,
							UnitRef: 4,
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
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.5},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 3,
							UnitRef: 4,
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
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "staleNaN"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: math.Float64frombits(value.StaleNaN)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 3,
							UnitRef: 4,
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
			converter := newPrometheusConverterV2(Settings{})
			unitNamer := otlptranslator.UnitNamer{}
			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: unitNamer.Build(metric.Unit()),
			}
			converter.addGaugeNumberDataPoints(metric.Gauge().DataPoints(), pcommon.NewResource(), settings, metric.Name(), m)
			w := tt.want()

			diff := cmp.Diff(w, converter.unique, cmpopts.EquateNaNs())
			assert.Empty(t, diff)
		})
	}
}

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
		labels := []prompb.Label{
			{Name: model.MetricNameLabel, Value: "test"},
		}
		return map[uint64]*writev2.TimeSeries{
			timeSeriesSignature(labels): {
				LabelsRefs: []uint32{1, 2},
				Samples: []writev2.Sample{
					{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1},
					{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 2},
				},
				Metadata: writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
					HelpRef: 3,
					UnitRef: 4,
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

	converter := newPrometheusConverterV2(Settings{})
	unitNamer := otlptranslator.UnitNamer{}
	m1 := metadata{
		Type: otelMetricTypeToPromMetricTypeV2(metric1),
		Help: metric1.Description(),
		Unit: unitNamer.Build(metric1.Unit()),
	}
	converter.addGaugeNumberDataPoints(metric1.Gauge().DataPoints(), pcommon.NewResource(), settings, metric1.Name(), m1)

	m2 := metadata{
		Type: otelMetricTypeToPromMetricTypeV2(metric2),
		Help: metric2.Description(),
		Unit: unitNamer.Build(metric2.Unit()),
	}
	converter.addGaugeNumberDataPoints(metric2.Gauge().DataPoints(), pcommon.NewResource(), settings, metric2.Name(), m2)

	assert.Equal(t, want(), converter.unique)
}
