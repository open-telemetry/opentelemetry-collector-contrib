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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
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
	requireMetricsOk(t, MetricsData(zap.NewNop(), summary, metadata, "", ValidMetricGroups))

	// Disable all groups
	require.Equal(t, 0, len(MetricsData(zap.NewNop(), summary, metadata, "", map[MetricGroup]bool{})))
}

func requireMetricsOk(t *testing.T, mds []pdata.Metrics) {
	for _, md := range mds {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			requireResourceOk(t, rm.Resource())
			for j := 0; j < rm.InstrumentationLibraryMetrics().Len(); j++ {
				ilm := rm.InstrumentationLibraryMetrics().At(j)
				for k := 0; k < ilm.Metrics().Len(); k++ {
					requireMetricOk(t, ilm.Metrics().At(k))
				}
			}
		}
	}
}

func requireMetricOk(t *testing.T, m pdata.Metric) {
	require.NotZero(t, m.Name())
	require.NotEqual(t, pdata.MetricDataTypeNone, m.DataType())

	switch m.DataType() {
	case pdata.MetricDataTypeGauge:
		gauge := m.Gauge()
		for i := 0; i < gauge.DataPoints().Len(); i++ {
			dp := gauge.DataPoints().At(i)
			require.NotZero(t, dp.Timestamp())
			requirePointOk(t, dp)
		}
	case pdata.MetricDataTypeSum:
		sum := m.Sum()
		require.True(t, sum.IsMonotonic())
		require.Equal(t, pdata.MetricAggregationTemporalityCumulative, sum.AggregationTemporality())
		for i := 0; i < sum.DataPoints().Len(); i++ {
			dp := sum.DataPoints().At(i)
			// Start time is required for cumulative metrics. Make assertions
			// around start time only when dealing with one or when it is set.
			require.NotZero(t, dp.StartTimestamp())
			require.NotZero(t, dp.Timestamp())
			require.Less(t, dp.StartTimestamp(), dp.Timestamp())
			requirePointOk(t, dp)
		}
	}
}

func requirePointOk(t *testing.T, point pdata.NumberDataPoint) {
	require.NotZero(t, point.Timestamp())
	require.NotEqual(t, pdata.MetricValueTypeNone, point.Type())
}

func requireResourceOk(t *testing.T, resource pdata.Resource) {
	require.NotZero(t, resource.Attributes().Len())
}

func TestWorkingSetMem(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.working_set")
	requireContains(t, metrics, "container.memory.working_set")

	nodeWsMetrics := metrics["k8s.node.memory.working_set"]
	value := nodeWsMetrics[0].Gauge().DataPoints().At(0).IntVal()
	require.Equal(t, int64(1234567890), value)
}

func TestPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.page_faults")
	requireContains(t, metrics, "container.memory.page_faults")

	nodePageFaults := metrics["k8s.node.memory.page_faults"]
	value := nodePageFaults[0].Gauge().DataPoints().At(0).IntVal()
	require.Equal(t, int64(12345), value)
}

func TestMajorPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.major_page_faults")
	requireContains(t, metrics, "container.memory.major_page_faults")

	nodePageFaults := metrics["k8s.node.memory.major_page_faults"]
	value := nodePageFaults[0].Gauge().DataPoints().At(0).IntVal()
	require.Equal(t, int64(12), value)
}

func requireContains(t *testing.T, metrics map[string][]pdata.Metric, metricName string) {
	_, found := metrics[metricName]
	require.True(t, found)
}

func indexedFakeMetrics() map[string][]pdata.Metric {
	mds := fakeMetrics()
	metrics := make(map[string][]pdata.Metric)
	for _, md := range mds {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.InstrumentationLibraryMetrics().Len(); j++ {
				ilm := rm.InstrumentationLibraryMetrics().At(j)
				for k := 0; k < ilm.Metrics().Len(); k++ {
					m := ilm.Metrics().At(k)
					metricName := m.Name()
					list := metrics[metricName]
					list = append(list, m)
					metrics[metricName] = list
				}
			}
		}
	}
	return metrics
}

func fakeMetrics() []pdata.Metrics {
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
