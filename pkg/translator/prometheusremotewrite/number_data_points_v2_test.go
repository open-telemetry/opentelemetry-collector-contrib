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
	"github.com/stretchr/testify/require"
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
			err := converter.addGaugeNumberDataPoints(metric.Gauge().DataPoints(), pcommon.NewResource(), pcommon.NewInstrumentationScope(), settings, metric.Name(), m)
			require.NoError(t, err)
			w := tt.want()

			diff := cmp.Diff(w, converter.unique, cmpopts.EquateNaNs())
			assert.Empty(t, diff)
			assert.Empty(t, converter.conflicts)
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
	err := converter.addGaugeNumberDataPoints(metric1.Gauge().DataPoints(), pcommon.NewResource(), pcommon.NewInstrumentationScope(), settings, metric1.Name(), m1)
	require.NoError(t, err)

	m2 := metadata{
		Type: otelMetricTypeToPromMetricTypeV2(metric2),
		Help: metric2.Description(),
		Unit: unitNamer.Build(metric2.Unit()),
	}
	err = converter.addGaugeNumberDataPoints(metric2.Gauge().DataPoints(), pcommon.NewResource(), pcommon.NewInstrumentationScope(), settings, metric2.Name(), m2)
	require.NoError(t, err)

	assert.Equal(t, want(), converter.unique)
	assert.Empty(t, converter.conflicts)
}

func TestPrometheusConverterV2_addSumNumberDataPoints(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[uint64]*writev2.TimeSeries
	}{
		{
			name: "sum",
			metric: func() pmetric.Metric {
				return getIntSumMetric(
					"test",
					pcommon.NewMap(),
					pmetric.AggregationTemporalityCumulative,
					1, uint64(ts.AsTime().UnixNano()),
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
							{
								Value:     1,
								Timestamp: convertTimeStamp(ts),
							},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "sum with exemplars",
			metric: func() pmetric.Metric {
				m := getIntSumMetric(
					"test",
					pcommon.NewMap(),
					pmetric.AggregationTemporalityCumulative,
					1, uint64(ts.AsTime().UnixNano()),
				)
				m.Sum().DataPoints().At(0).Exemplars().AppendEmpty().SetDoubleValue(2)
				return m
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{{
							Value:     1,
							Timestamp: convertTimeStamp(ts),
						}},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 0,
						},
						// TODO add exemplars
						/*Exemplars: []writev2.Exemplar{
							{Value: 2},
						},*/
					},
				}
			},
		},
		{
			name: "monotonic cumulative sum with start timestamp",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(true)

				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1)
				dp.SetTimestamp(ts)
				dp.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 1, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "monotonic cumulative sum with no start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(true)

				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "non-monotonic cumulative sum with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(false)

				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
							HelpRef: 0,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()
			converter := newPrometheusConverterV2(Settings{})
			unitNamer := otlptranslator.UnitNamer{}

			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: unitNamer.Build(metric.Unit()),
			}

			err := converter.addSumNumberDataPoints(
				metric.Sum().DataPoints(),
				pcommon.NewResource(),
				pcommon.NewInstrumentationScope(),
				metric,
				Settings{},
				metric.Name(),
				m,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want(), converter.unique)
			assert.Empty(t, converter.conflicts)
		})
	}
}
