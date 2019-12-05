// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectdreceiver

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeEvent(t *testing.T) {
	m1 := []*metricspb.Metric{}

	jsonData, err := loadFromJSON("./testdata/event.json")
	require.NoError(t, err)

	records := []collectDRecord{}
	err = json.Unmarshal(jsonData, &records)
	require.NoError(t, err)

	for _, r := range records {
		m2, err := r.appendToMetrics(m1, map[string]string{})
		assert.NoError(t, err)
		assert.Len(t, m2, 0)
	}
}

func loadFromJSON(path string) ([]byte, error) {
	var body []byte
	jsonFile, err := os.Open(path)
	if err != nil {
		return body, err
	}
	defer jsonFile.Close()

	return ioutil.ReadAll(jsonFile)
}

func TestDecodeMetrics(t *testing.T) {
	metrics := []*metricspb.Metric{}

	jsonData, err := loadFromJSON("./testdata/collectd.json")
	require.NoError(t, err)

	records := []collectDRecord{}
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
					{Value: "fake"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "fake"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "load"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "load"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "load"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "memory"},
					{Value: "i-b13d1e5f"},
					{Value: "value"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
				LabelValues: []*metricspb.LabelValue{
					{Value: "value"},
					{Value: "df"},
					{Value: "dev"},
					{Value: "i-b13d1e5f"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "mwp-signalbox"},
					{Value: "value"},
					{Value: "tail"},
					{Value: "analytics"},
					{Value: "v1"},
					{Value: "v2"},
					{Value: "b"},
					{Value: "x"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "mwp-signalbox"},
					{Value: "value"},
					{Value: "tail"},
					{Value: "analytics"},
					{Value: "v1"},
					{Value: "v2"},
					{Value: "b"},
					{Value: "x"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
					{Value: "some-host"},
					{Value: "value"},
					{Value: "dogstatsd"},
					{Value: "dev"},
					{Value: "v1"},
				},
				Points: []*metricspb.Point{
					{
						Timestamp: &timestamp.Timestamp{
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
