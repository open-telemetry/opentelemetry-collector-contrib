// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"maps"
	"slices"
	"testing"
	"time"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:         "",
		ExternalLabels:    nil,
		DisableTargetInfo: false,
		AddMetricSuffixes: false,
		SendMetadata:      false,
	}

	ts := uint64(time.Now().UnixNano())
	payload := createExportRequest(5, 0, 1, 3, 0, pcommon.Timestamp(ts))
	want := []*writev2.TimeSeries{
		{
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
			Samples: []writev2.Sample{
				{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
			},
		},
		{
			LabelsRefs: []uint32{1, 9, 3, 4, 5, 6, 7, 8},
			Samples: []writev2.Sample{
				{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
			},
		},
	}
	wantedSymbols := []string{"", "series_name_2", "value-2", "series_name_3", "value-3", "__name__", "gauge_1", "series_name_1", "value-1", "sum_1"}
	tsMap, symbolsTable, err := FromMetricsV2(payload.Metrics(), settings)
	require.NoError(t, err)
	require.ElementsMatch(t, want, slices.Collect(maps.Values(tsMap)))
	require.ElementsMatch(t, wantedSymbols, symbolsTable.Symbols())
}

func TestFromMetricsV2_Histograms(t *testing.T) {
	settings := Settings{
		Namespace:         "",
		ExternalLabels:    nil,
		DisableTargetInfo: false,
		AddMetricSuffixes: false,
		SendMetadata:      false,
	}

	ts := uint64(time.Now().UnixNano())
	// want := []*writev2.TimeSeries{
	// 	{
	// 		LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
	// 		Samples: []writev2.Sample{
	// 			{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
	// 		},
	// 	},
	// 	{
	// 		LabelsRefs: []uint32{1, 9, 3, 4, 5, 6, 7, 8},
	// 		Samples: []writev2.Sample{
	// 			{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
	// 		},
	// 	},
	// }
	// wantedSymbols := []string{"", "series_name_2", "value-2", "series_name_3", "value-3", "__name__", "gauge_1", "series_name_1", "value-1", "sum_1"}
	metric := createExportRequest(1, 1, 0, 1, 1, pcommon.Timestamp(ts))
	tsMap, symbolsTable, err := FromMetricsV2(metric.Metrics(), settings)
	require.NoError(t, err)

	t.Logf("tsMap: %v\n", tsMap)
	t.Logf("symbolsTable %v\n", symbolsTable)

}

// func TestPrometheusConverterV2_AddHistogramDataPoints(t *testing.T) {
// 	ts := pcommon.Timestamp(time.Now().UnixNano())
// 	tests := []struct {
// 		name   string
// 		metric func() pmetric.Metric
// 		want   func() map[uint64]*writev2.TimeSeries
// 	}{
// 		{
// 			name: "histogram with start time",
// 			metric: func() pmetric.Metric {
// 				metric := pmetric.NewMetric()
// 				metric.SetName("test_hist")
// 				metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
// 				pt := metric.Histogram().DataPoints().AppendEmpty()
// 				pt.SetTimestamp(ts)
// 				pt.SetStartTimestamp(ts)
// 				return pmetric.NewMetric()
// 			},
// 			want: func() map[uint64]*writev2.TimeSeries {
// 				labels := []prompb.Label{
// 					{Name: model.MetricNameLabel, Value: "test_hist" + countStr},
// 				}

// 				infLabels := []prompb.Label{
// 					{Name: model.MetricNameLabel, Value: "test_hist_bucket"},
// 					{Name: model.BucketLabel, Value: "+Inf"},
// 				}

// 				return map[uint64]*writev2.TimeSeries{
// 					timeSeriesSignature(infLabels): {
// 						LabelsRefs: ,
// 					}
// 				}
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			metric := tt.metric()

// 			converter := newPrometheusConverterV2()
// 			converter.addHistogramDataPoints(
// 				metric.Histogram().DataPoints(),
// 				pcommon.NewResource(),
// 				Settings{},
// 				metric.Name(),
// 			)

// 			t.Logf("%v", converter.unique)
// 		})
// 	}
// }
