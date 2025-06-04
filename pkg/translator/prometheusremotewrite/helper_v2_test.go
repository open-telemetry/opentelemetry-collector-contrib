// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestPrometheusConverterV2_AddHistogramDataPoints(t *testing.T) {
	ts := pcommon.Timestamp(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   func() map[uint64]*writev2.TimeSeries
	}{
		{
			name: "histogram with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)
				pt.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(infLabels): {
						LabelsRefs: []uint32{1, 3, 4, 5},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "histogram without start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(infLabels): {
						LabelsRefs: []uint32{1, 3, 4, 5},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
				}
			},
		},
		{
			name: "histogram with exportCreatedMetricGate enabled",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.Histogram().DataPoints().AppendEmpty()
				pt.SetTimestamp(ts)
				pt.SetStartTimestamp(ts)

				return metric
			},
			want: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
				}
				infLabels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
					{Name: model.BucketLabel, Value: "+Inf"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(infLabels): {
						LabelsRefs: []uint32{1, 3, 4, 5},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2},
						Samples: []writev2.Sample{
							{Value: 0, Timestamp: convertTimeStamp(ts)},
						},
						Metadata: writev2.Metadata{
							Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							HelpRef: 0,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue := exportCreatedMetricGate.IsEnabled()
			testutil.SetFeatureGateForTest(t, exportCreatedMetricGate, true)
			defer testutil.SetFeatureGateForTest(t, exportCreatedMetricGate, oldValue)

			metric := tt.metric()
			converter := newPrometheusConverterV2()
			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: prometheustranslator.BuildCompliantPrometheusUnit(metric.Unit()),
			}
			converter.addHistogramDataPoints(
				metric.Histogram().DataPoints(),
				pcommon.NewResource(),
				Settings{},
				metric.Name(),
				m,
			)

			assert.Equal(t, tt.want(), converter.unique)
			// TODO check when conflicts handling is implemented
			// assert.Empty(t, converter.conflicts)
		})
	}
}
