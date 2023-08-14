// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAddSingleGaugeNumberDataPoint(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[string]*prompb.TimeSeries
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeGauge.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{
								Value:     1,
								Timestamp: convertTimeStamp(pcommon.Timestamp(ts)),
							}},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			gotSeries := make(map[string]*prompb.TimeSeries)

			for x := 0; x < metric.Gauge().DataPoints().Len(); x++ {
				addSingleGaugeNumberDataPoint(
					metric.Gauge().DataPoints().At(x),
					pcommon.NewResource(),
					metric,
					Settings{},
					gotSeries,
				)
			}
			assert.Equal(t, tt.want(), gotSeries)
		})
	}
}

func TestAddSingleSumNumberDataPoint(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[string]*prompb.TimeSeries
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{
								Value:     1,
								Timestamp: convertTimeStamp(ts),
							}},
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{{
							Value:     1,
							Timestamp: convertTimeStamp(ts),
						}},
						Exemplars: []prompb.Exemplar{
							{Value: 2},
						},
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				createdLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum" + createdSuffix},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 1, Timestamp: convertTimeStamp(ts)},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &createdLabels): {
						Labels: createdLabels,
						Samples: []prompb.Sample{
							{Value: float64(convertTimeStamp(ts))},
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
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
			want: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_sum"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeSum.String(), &labels): {
						Labels: labels,
						Samples: []prompb.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			got := make(map[string]*prompb.TimeSeries)

			for x := 0; x < metric.Sum().DataPoints().Len(); x++ {
				addSingleSumNumberDataPoint(
					metric.Sum().DataPoints().At(x),
					pcommon.NewResource(),
					metric,
					Settings{ExportCreatedMetric: true},
					got,
				)
			}
			assert.Equal(t, tt.want(), got)
		})
	}
}
