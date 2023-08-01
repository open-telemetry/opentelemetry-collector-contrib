// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDecodeEvent(t *testing.T) {
	var m1 []*metricspb.Metric

	jsonData, err := os.ReadFile(filepath.Join("testdata", "event.json"))
	require.NoError(t, err)

	var records []collectDRecord
	err = json.Unmarshal(jsonData, &records)
	require.NoError(t, err)

	for _, r := range records {
		m2, err := r.appendToMetrics(m1, map[string]string{})
		assert.NoError(t, err)
		assert.Len(t, m2, 0)
	}
}

func TestDecodeMetrics(t *testing.T) {
	var metrics []*metricspb.Metric

	jsonData, err := os.ReadFile(filepath.Join("testdata", "collectd.json"))
	require.NoError(t, err)

	var records []collectDRecord
	err = json.Unmarshal(jsonData, &records)
	require.NoError(t, err)

	for _, r := range records {
		metrics, err = r.appendToMetrics(metrics, map[string]string{})
		assert.NoError(t, err)
	}
	assert.Equal(t, 10, len(metrics))

	assertMetricsAreEqual(t, wantMetricsData, metrics)
}

var wantMetricsData = []*metricspb.Metric{
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "load.low",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "fake", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 0.2},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "load.high",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "fake", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 0.9},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "load.shortterm",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "load", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 0.37},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "load.midterm",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "load", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 0.61},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "load.longterm",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "load", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 0.76},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "memory.used",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "plugin"},
				{Key: "host"},
				{Key: "dsname"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "memory", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
					{Value: "value", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   496000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 1.52431e+09},
					},
				},
			},
		},
	},
	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "df_complex.free",
			Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "dsname"},
				{Key: "plugin"},
				{Key: "plugin_instance"},
				{Key: "host"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				StartTimestamp: &timestamppb.Timestamp{
					Seconds: 1415062567,
					Nanos:   494999808,
				},
				LabelValues: []*metricspb.LabelValue{
					{Value: "value", HasValue: true},
					{Value: "df", HasValue: true},
					{Value: "dev", HasValue: true},
					{Value: "i-b13d1e5f", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1415062577,
							Nanos:   494999808,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 1.9626e+09},
					},
				},
			},
		},
	},

	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "memory.old_gen_end",
			Type: metricspb.MetricDescriptor_GAUGE_INT64,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "host"},
				{Key: "dsname"},
				{Key: "plugin"},
				{Key: "plugin_instance"},
				{Key: "k1"},
				{Key: "k2"},
				{Key: "a"},
				{Key: "f"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "mwp-signalbox", HasValue: true},
					{Value: "value", HasValue: true},
					{Value: "tail", HasValue: true},
					{Value: "analytics", HasValue: true},
					{Value: "v1", HasValue: true},
					{Value: "v2", HasValue: true},
					{Value: "b", HasValue: true},
					{Value: "x", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1434477504,
							Nanos:   484000000,
						},
						Value: &metricspb.Point_Int64Value{Int64Value: 26790},
					},
				},
			},
		},
	},

	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "memory.total_heap_space",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "host"},
				{Key: "dsname"},
				{Key: "plugin"},
				{Key: "plugin_instance"},
				{Key: "k1"},
				{Key: "k2"},
				{Key: "a"},
				{Key: "f"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "mwp-signalbox", HasValue: true},
					{Value: "value", HasValue: true},
					{Value: "tail", HasValue: true},
					{Value: "analytics", HasValue: true},
					{Value: "v1", HasValue: true},
					{Value: "v2", HasValue: true},
					{Value: "b", HasValue: true},
					{Value: "x", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1434477504,
							Nanos:   484000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 1.03552e+06},
					},
				},
			},
		},
	},

	{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: "gauge.page.loadtime",
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: []*metricspb.LabelKey{
				{Key: "host"},
				{Key: "dsname"},
				{Key: "plugin"},
				{Key: "env"},
				{Key: "k1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "some-host", HasValue: true},
					{Value: "value", HasValue: true},
					{Value: "dogstatsd", HasValue: true},
					{Value: "dev", HasValue: true},
					{Value: "v1", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamppb.Timestamp{
							Seconds: 1434477504,
							Nanos:   484000000,
						},
						Value: &metricspb.Point_DoubleValue{DoubleValue: 12},
					},
				},
			},
		},
	},
}

func TestLabelsFromName(t *testing.T) {
	tests := []struct {
		name           string
		wantMetricName string
		wantLabels     map[string]string
	}{
		{
			name:           "simple",
			wantMetricName: "simple",
		},
		{
			name:           "single[k=v]",
			wantMetricName: "single",
			wantLabels: map[string]string{
				"k": "v",
			},
		},
		{
			name:           "a.b.c.[k=v].d",
			wantMetricName: "a.b.c..d",
			wantLabels: map[string]string{
				"k": "v",
			},
		},
		{
			name:           "a.b[k0=v0,k1=v1,k2=v2].c",
			wantMetricName: "a.b.c",
			wantLabels: map[string]string{
				"k0": "v0", "k1": "v1", "k2": "v2",
			},
		},
		{
			name:           "empty[]",
			wantMetricName: "empty[]",
		},
		{
			name:           "mal.formed[k_no_sep]",
			wantMetricName: "mal.formed[k_no_sep]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMetricName, gotLabels := LabelsFromName(&tt.name)
			assert.Equal(t, tt.wantMetricName, gotMetricName)
			assert.Equal(t, tt.wantLabels, gotLabels)
		})
	}
}

func createPtrFloat64(v float64) *float64 {
	return &v
}

func TestStartTimestamp(t *testing.T) {
	tests := []struct {
		name                 string
		record               collectDRecord
		metricDescriptorType metricspb.MetricDescriptor_Type
		wantStartTimestamp   *timestamppb.Timestamp
	}{
		{
			name: "metric type cumulative distribution",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
			wantStartTimestamp: &timestamppb.Timestamp{
				Seconds: 5,
				Nanos:   0,
			},
		},
		{
			name: "metric type cumulative double",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
			wantStartTimestamp: &timestamppb.Timestamp{
				Seconds: 5,
				Nanos:   0,
			},
		},
		{
			name: "metric type cumulative int64",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
			wantStartTimestamp: &timestamppb.Timestamp{
				Seconds: 5,
				Nanos:   0,
			},
		},
		{
			name: "metric type non-cumulative gauge distribution",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: metricspb.MetricDescriptor_GAUGE_DISTRIBUTION,
			wantStartTimestamp:   nil,
		},
		{
			name: "metric type non-cumulative gauge int64",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: metricspb.MetricDescriptor_GAUGE_INT64,
			wantStartTimestamp:   nil,
		},
		{
			name: "metric type non-cumulativegauge double",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			wantStartTimestamp:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStartTimestamp := tc.record.startTimestamp(tc.metricDescriptorType)
			assert.Equal(t, tc.wantStartTimestamp, gotStartTimestamp)
		})
	}
}
