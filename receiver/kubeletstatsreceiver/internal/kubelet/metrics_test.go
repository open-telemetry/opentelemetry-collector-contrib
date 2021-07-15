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

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeRestClient struct {
}

func (f fakeRestClient) StatsSummary() ([]byte, error) {
	return ioutil.ReadFile("../../testdata/stats-summary.json")
}

func (f fakeRestClient) Pods() ([]byte, error) {
	return ioutil.ReadFile("../../testdata/pods.json")
}

func TestMetricAccumulator(t *testing.T) {
	rc := &fakeRestClient{}
	statsProvider := NewStatsProvider(rc)
	summary, _ := statsProvider.StatsSummary()
	metadataProvider := NewMetadataProvider(rc)
	podsMetadata, _ := metadataProvider.Pods()
	metadata := NewMetadata([]MetadataLabel{MetadataLabelContainerID}, podsMetadata, nil)
	requireMetricsDataOk(t, MetricsData(zap.NewNop(), summary, metadata, "", ValidMetricGroups))

	// Disable all groups
	require.Equal(t, 0, len(MetricsData(zap.NewNop(), summary, metadata, "", map[MetricGroup]bool{})))
}

func requireMetricsDataOk(t *testing.T, mds []*agentmetricspb.ExportMetricsServiceRequest) {
	for _, md := range mds {
		requireResourceOk(t, md.Resource)
		for _, metric := range md.Metrics {
			requireDescriptorOk(t, metric.MetricDescriptor)
			isCumulativeType := isCumulativeType(metric.GetMetricDescriptor().Type)
			for _, ts := range metric.Timeseries {
				// Start time is required for cumulative metrics. Make assertions
				// around start time only when dealing with one or when it is set.
				shouldCheckStartTime := isCumulativeType || ts.StartTimestamp != nil
				requireTimeSeriesOk(t, ts, shouldCheckStartTime)
			}
		}
	}
}

func requireTimeSeriesOk(t *testing.T, ts *metricspb.TimeSeries, shouldCheckStartTime bool) {
	if shouldCheckStartTime {
		require.True(t, ts.StartTimestamp.Seconds > 0)
	}

	for _, point := range ts.Points {
		requirePointOk(t, point, ts, shouldCheckStartTime)
	}
}

func requirePointOk(t *testing.T, point *metricspb.Point, ts *metricspb.TimeSeries, shouldCheckStartTime bool) {
	if shouldCheckStartTime {
		require.True(t, point.Timestamp.Seconds > ts.StartTimestamp.Seconds)
	}
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

func isCumulativeType(typ metricspb.MetricDescriptor_Type) bool {
	return typ == metricspb.MetricDescriptor_CUMULATIVE_DOUBLE ||
		typ == metricspb.MetricDescriptor_CUMULATIVE_INT64 ||
		typ == metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION
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

func TestWorkingSetMem(t *testing.T) {
	metrics := indexedFakeMetrics()
	nodeWsMetrics := metrics["k8s.node.memory.working_set"]
	wsMetric := nodeWsMetrics[0]
	value := wsMetric.Timeseries[0].Points[0].Value
	ptv := value.(*metricspb.Point_Int64Value)
	require.Equal(t, int64(1234567890), ptv.Int64Value)
	requireContains(t, metrics, "k8s.pod.memory.working_set")
	requireContains(t, metrics, "container.memory.working_set")
}

func TestPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	nodePageFaults := metrics["k8s.node.memory.page_faults"]
	value := nodePageFaults[0].Timeseries[0].Points[0].Value
	ptv := value.(*metricspb.Point_Int64Value)
	require.Equal(t, int64(12345), ptv.Int64Value)
	requireContains(t, metrics, "k8s.pod.memory.page_faults")
	requireContains(t, metrics, "container.memory.page_faults")
}

func TestMajorPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	nodePageFaults := metrics["k8s.node.memory.major_page_faults"]
	value := nodePageFaults[0].Timeseries[0].Points[0].Value
	ptv := value.(*metricspb.Point_Int64Value)
	require.Equal(t, int64(12), ptv.Int64Value)
	requireContains(t, metrics, "k8s.pod.memory.major_page_faults")
	requireContains(t, metrics, "container.memory.major_page_faults")
}

func requireContains(t *testing.T, metrics map[string][]*metricspb.Metric, metricName string) {
	_, found := metrics[metricName]
	require.True(t, found)
}

func indexedFakeMetrics() map[string][]*metricspb.Metric {
	mds := fakeMetrics()
	metrics := make(map[string][]*metricspb.Metric)
	for _, md := range mds {
		for _, metric := range md.Metrics {
			metricName := metric.MetricDescriptor.Name
			list := metrics[metricName]
			list = append(list, metric)
			metrics[metricName] = list
		}
	}
	return metrics
}

func fakeMetrics() []*agentmetricspb.ExportMetricsServiceRequest {
	rc := &fakeRestClient{}
	statsProvider := NewStatsProvider(rc)
	summary, _ := statsProvider.StatsSummary()
	mgs := map[MetricGroup]bool{
		ContainerMetricGroup: true,
		PodMetricGroup:       true,
		NodeMetricGroup:      true,
	}
	return MetricsData(zap.NewNop(), summary, Metadata{}, "foo", mgs)
}
