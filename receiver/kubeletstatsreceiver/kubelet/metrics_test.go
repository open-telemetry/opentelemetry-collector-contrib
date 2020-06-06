// Copyright 2020, OpenTelemetry Authors
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

package kubelet

import (
	"io/ioutil"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
)

type fakeRestClient struct {
}

func (f fakeRestClient) StatsSummary() ([]byte, error) {
	return ioutil.ReadFile("../testdata/stats-summary.json")
}

func (f fakeRestClient) Pods() ([]byte, error) {
	return ioutil.ReadFile("../testdata/pods.json")
}

func TestMetricAccumulator(t *testing.T) {
	provider := NewStatsProvider(&fakeRestClient{})
	summary, _ := provider.StatsSummary()
	requireMetricsDataOk(t, MetricsData(summary, ""))
}

func requireMetricsDataOk(t *testing.T, mds []*consumerdata.MetricsData) {
	for _, md := range mds {
		requireResourceOk(t, md.Resource)
		for _, metric := range md.Metrics {
			requireDescriptorOk(t, metric.MetricDescriptor)
			for _, ts := range metric.Timeseries {
				requireTimeSeriesOk(t, ts)
			}
		}
	}
}

func requireTimeSeriesOk(t *testing.T, ts *metricspb.TimeSeries) {
	require.True(t, ts.StartTimestamp.Seconds > 0)
	for _, point := range ts.Points {
		requirePointOk(t, point, ts)
	}
}

func requirePointOk(t *testing.T, point *metricspb.Point, ts *metricspb.TimeSeries) {
	require.True(t, point.Timestamp.Seconds > ts.StartTimestamp.Seconds)
	require.NotNil(t, point.Value)
}

func requireDescriptorOk(t *testing.T, desc *metricspb.MetricDescriptor) {
	require.True(t, desc.Name != "")
	require.True(t, desc.Type != metricspb.MetricDescriptor_UNSPECIFIED)
}

func requireResourceOk(t *testing.T, resource *resourcepb.Resource) {
	require.True(t, resource.Type != "")
	require.NotNil(t, resource.Labels)
}

func TestLabels(t *testing.T) {
	labelKeys, labelValues := labels(
		map[string]string{"mykey": "myval"},
		map[string]string{"mykey": "mydesc"},
	)
	labelKey := labelKeys[0]
	require.Equal(t, labelKey.Key, "mykey")
	require.Equal(t, labelKey.Description, "mydesc")
	labelValue := labelValues[0]
	require.Equal(t, labelValue.Value, "myval")
	require.True(t, labelValue.HasValue)
}
