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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
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
	k8sMetadata := NewMetadata([]MetadataLabel{MetadataLabelContainerID}, podsMetadata, nil)
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings())
	ScrapeMetrics(zap.NewNop(), summary, k8sMetadata, ValidMetricGroups, mb)
	requireMetricsOk(t, mb.Emit())

	// Disable all groups
	ScrapeMetrics(zap.NewNop(), summary, k8sMetadata, map[MetricGroup]bool{}, mb)
	require.Equal(t, 0, mb.Emit().ResourceMetrics().Len())
}

func requireMetricsOk(t *testing.T, md pmetric.Metrics) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		requireResourceOk(t, rm.Resource())
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			require.Equal(t, "otelcol/kubeletstatsreceiver", ilm.Scope().Name())
			for k := 0; k < ilm.Metrics().Len(); k++ {
				requireMetricOk(t, ilm.Metrics().At(k))
			}
		}
	}
}

func requireMetricOk(t *testing.T, m pmetric.Metric) {
	require.NotZero(t, m.Name())
	require.NotEqual(t, pmetric.MetricDataTypeNone, m.DataType())
	switch m.DataType() {
	case pmetric.MetricDataTypeGauge:
		gauge := m.Gauge()
		for i := 0; i < gauge.DataPoints().Len(); i++ {
			dp := gauge.DataPoints().At(i)
			require.NotZero(t, dp.Timestamp())
			requirePointOk(t, dp)
		}
	case pmetric.MetricDataTypeSum:
		sum := m.Sum()
		require.True(t, sum.IsMonotonic())
		require.Equal(t, pmetric.MetricAggregationTemporalityCumulative, sum.AggregationTemporality())
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

func requirePointOk(t *testing.T, point pmetric.NumberDataPoint) {
	require.NotZero(t, point.Timestamp())
	require.NotEqual(t, pmetric.NumberDataPointValueTypeNone, point.ValueType())
}

func requireResourceOk(t *testing.T, resource pcommon.Resource) {
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

func requireContains(t *testing.T, metrics map[string][]pmetric.Metric, metricName string) {
	_, found := metrics[metricName]
	require.True(t, found)
}

func indexedFakeMetrics() map[string][]pmetric.Metric {
	md := fakeMetrics()
	metrics := make(map[string][]pmetric.Metric)
	for i := 0; i < fakeMetrics().ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				m := ilm.Metrics().At(k)
				metricName := m.Name()
				list := metrics[metricName]
				list = append(list, m)
				metrics[metricName] = list
			}
		}
	}
	return metrics
}

func fakeMetrics() pmetric.Metrics {
	rc := &fakeRestClient{}
	statsProvider := NewStatsProvider(rc)
	summary, _ := statsProvider.StatsSummary()
	mgs := map[MetricGroup]bool{
		ContainerMetricGroup: true,
		PodMetricGroup:       true,
		NodeMetricGroup:      true,
	}
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings())
	ScrapeMetrics(zap.NewNop(), summary, Metadata{}, mgs, mb)
	return mb.Emit()
}
