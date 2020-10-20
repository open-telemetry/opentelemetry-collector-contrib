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
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/testutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type metricLabel struct {
	key   *metricspb.LabelKey
	value *metricspb.LabelValue
}

func TestNewReceiver(t *testing.T) {
	type args struct {
		addr         string
		timeout      time.Duration
		attrsPrefix  string
		nextConsumer consumer.MetricsConsumer
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "nil nextConsumer",
			args: args{
				addr:        ":0",
				timeout:     defaultTimeout,
				attrsPrefix: "default_attr_",
			},
			wantErr: errNilNextConsumer,
		},
		{
			name: "happy path",
			args: args{
				addr:         ":0",
				timeout:      defaultTimeout,
				attrsPrefix:  "default_attr_",
				nextConsumer: consumertest.NewMetricsNop(),
			},
		},
	}
	logger := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newCollectdReceiver(logger, tt.args.addr, time.Second*10, "", tt.args.nextConsumer)
			if err != tt.wantErr {
				t.Errorf("newCollectdReceiver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCollectDServer(t *testing.T) {
	const endpoint = "localhost:8081"
	defaultAttrsPrefix := "dap_"

	type testCase struct {
		name         string
		queryParams  string
		requestBody  string
		responseCode int
		wantData     []consumerdata.MetricsData
	}

	testCases := []testCase{{
		name:        "valid-request-body",
		queryParams: "dap_attr1=attr1val",
		requestBody: `[
    {
        "dsnames": [
            "value"
        ],
        "dstypes": [
            "derive"
        ],
        "host": "i-b13d1e5f",
        "interval": 10.0,
        "plugin": "memory",
        "plugin_instance": "",
        "time": 1415062577.4949999,
        "type": "memory",
        "type_instance": "free",
        "values": [
            2.1474
        ]
    }
]`,
		responseCode: 200,
		wantData: []consumerdata.MetricsData{{
			Metrics: []*metricspb.Metric{{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "memory.free",
					Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "plugin"},
						{Key: "host"},
						{Key: "dsname"},
						{Key: "attr1"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{{
					LabelValues: []*metricspb.LabelValue{
						{Value: "memory", HasValue: true},
						{Value: "i-b13d1e5f", HasValue: true},
						{Value: "value", HasValue: true},
						{Value: "attr1val", HasValue: true},
					},
					Points: []*metricspb.Point{{
						Timestamp: &timestamppb.Timestamp{Seconds: 1415062577, Nanos: 494999808},
						Value: &metricspb.Point_DoubleValue{
							DoubleValue: 2.1474,
						}},
					},
				}},
			}},
		}},
	}, {
		name:         "invalid-request-body",
		requestBody:  `invalid-body`,
		responseCode: 400,
		wantData:     []consumerdata.MetricsData{},
	}}

	sink := new(consumertest.MetricsSink)

	logger := zap.NewNop()
	cdr, err := newCollectdReceiver(logger, endpoint, defaultTimeout, defaultAttrsPrefix, sink)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	require.NoError(t, cdr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		err := cdr.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error stopping metrics reception: %v", err)
		}
	}()

	time.Sleep(time.Second)

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sink.Reset()
			req, err := http.NewRequest(
				"POST",
				"http://"+endpoint+"?"+tt.queryParams,
				bytes.NewBuffer([]byte(tt.requestBody)),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, tt.responseCode, resp.StatusCode)
			defer resp.Body.Close()

			if tt.responseCode != 200 {
				return
			}

			testutil.WaitFor(t, func() bool {
				return len(sink.AllMetrics()) == 1
			})
			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			got := internaldata.MetricsToOC(mds[0])
			assertMetricsDataAreEqual(t, got, tt.wantData)
		})
	}
}

func assertMetricsDataAreEqual(t *testing.T, metricsData1, metricsData2 []consumerdata.MetricsData) {
	if len(metricsData1) != len(metricsData2) {
		t.Errorf("metrics data length mismatch. got:\n%d\nwant:\n%d\n", len(metricsData1), len(metricsData2))
		return
	}

	for i := 0; i < len(metricsData1); i++ {
		md1, md2 := metricsData1[i], metricsData2[i]

		if !proto.Equal(md1.Node, md2.Node) {
			t.Errorf("metrics data nodes are not equal. got:\n%+v\nwant:\n%+v\n", md1.Node, md2.Node)
		}
		if !proto.Equal(md1.Resource, md2.Resource) {
			t.Errorf("metrics data resources are not equal. got:\n%+v\nwant:\n%+v\n", md1.Resource, md2.Resource)
		}

		assertMetricsAreEqual(t, md1.Metrics, md2.Metrics)
	}
}

func assertMetricsAreEqual(t *testing.T, metrics1, metrics2 []*metricspb.Metric) {
	if len(metrics1) != len(metrics2) {
		t.Errorf("metrics length mismatch. got:\n%d\nwant:\n%d\n", len(metrics1), len(metrics2))
		return
	}

	for i := 0; i < len(metrics1); i++ {
		m1, m2 := metrics1[i], metrics2[i]

		if !proto.Equal(m1.Resource, m2.Resource) {
			t.Errorf("metric resources are not equal. got:\n%+v\nwant:\n%+v\n", m1.Resource, m2.Resource)
		}

		md1, md2 := m1.MetricDescriptor, m2.MetricDescriptor
		assert.Equal(t, md1.Name, md2.Name)
		assert.Equal(t, md1.Type, md2.Type)
		assert.Equal(t, md1.Description, md2.Description)
		assert.Equal(t, md1.Unit, md2.Unit)

		if len(md1.LabelKeys) != len(md2.LabelKeys) {
			t.Errorf("label keys length mismatch. got:\n%d\nwant:\n%d\n", len(md1.LabelKeys), len(md2.LabelKeys))
			return
		}

		if len(m1.Timeseries) != len(m2.Timeseries) {
			t.Errorf("timeseries length mismatch. got:\n%d\nwant:\n%d\n", len(m1.Timeseries), len(m2.Timeseries))
			return
		}
		for i := 0; i < len(m1.Timeseries); i++ {
			t1, t2 := m1.Timeseries[i], m2.Timeseries[i]
			assert.Equal(t, t1.Points, t2.Points)
			assert.Equal(t, t1.StartTimestamp, t2.StartTimestamp)

			if len(t1.LabelValues) != len(t2.LabelValues) {
				t.Errorf("label values length mismatch. got:\n%d\nwant:\n%d\n", len(t1.LabelValues), len(t2.LabelValues))
				return
			}

			l1, l2 := labelsFromMetric(md1, t1), labelsFromMetric(md2, t2)
			if len(l1) != len(l2) {
				t.Errorf("labels length mismatch. got:\n%d\nwant:\n%d\n", len(l1), len(l2))
				return
			}
			assert.EqualValues(t, l1, l2)
		}
	}
}

func labelsFromMetric(md *metricspb.MetricDescriptor, ts *metricspb.TimeSeries) map[string]metricLabel {
	labels := map[string]metricLabel{}
	numValues := len(ts.LabelValues)
	for i, k := range md.LabelKeys {
		if i < numValues {
			labels[k.Key] = metricLabel{k, ts.LabelValues[i]}
			continue
		}
	}
	return labels
}
