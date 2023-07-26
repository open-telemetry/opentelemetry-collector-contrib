// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

type fakeRestClient struct {
}

func (f fakeRestClient) StatsSummary() ([]byte, error) {
	return os.ReadFile("../../testdata/stats-summary.json")
}

func (f fakeRestClient) Pods() ([]byte, error) {
	return os.ReadFile("../../testdata/pods.json")
}

func TestMetricAccumulator(t *testing.T) {
	rc := &fakeRestClient{}
	statsProvider := NewStatsProvider(rc)
	summary, _ := statsProvider.StatsSummary()
	metadataProvider := NewMetadataProvider(rc)
	podsMetadata, _ := metadataProvider.Pods()
	k8sMetadata := NewMetadata([]MetadataLabel{MetadataLabelContainerID}, podsMetadata, nil)
	rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder:      metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		PodMetricsBuilder:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		OtherMetricsBuilder:     metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
	}
	requireMetricsOk(t, MetricsData(zap.NewNop(), summary, k8sMetadata, ValidMetricGroups, rb, mbs))
	// Disable all groups
	mbs.NodeMetricsBuilder.Reset()
	mbs.PodMetricsBuilder.Reset()
	mbs.OtherMetricsBuilder.Reset()
	require.Equal(t, 0, len(MetricsData(zap.NewNop(), summary, k8sMetadata, map[MetricGroup]bool{}, rb, mbs)))
}

func requireMetricsOk(t *testing.T, mds []pmetric.Metrics) {
	for _, md := range mds {
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
}

func requireMetricOk(t *testing.T, m pmetric.Metric) {
	require.NotZero(t, m.Name())
	require.NotEqual(t, pmetric.MetricTypeEmpty, m.Type())
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		gauge := m.Gauge()
		for i := 0; i < gauge.DataPoints().Len(); i++ {
			dp := gauge.DataPoints().At(i)
			require.NotZero(t, dp.Timestamp())
			requirePointOk(t, dp)
		}
	case pmetric.MetricTypeSum:
		sum := m.Sum()
		require.True(t, sum.IsMonotonic())
		require.Equal(t, pmetric.AggregationTemporalityCumulative, sum.AggregationTemporality())
		for i := 0; i < sum.DataPoints().Len(); i++ {
			dp := sum.DataPoints().At(i)
			// Start time is required for cumulative metrics. Make assertions
			// around start time only when dealing with one or when it is set.
			require.NotZero(t, dp.StartTimestamp())
			require.NotZero(t, dp.Timestamp())
			require.Less(t, dp.StartTimestamp(), dp.Timestamp())
			requirePointOk(t, dp)
		}
	case pmetric.MetricTypeEmpty:
	case pmetric.MetricTypeHistogram:
	case pmetric.MetricTypeExponentialHistogram:
	case pmetric.MetricTypeSummary:
	}
}

func requirePointOk(t *testing.T, point pmetric.NumberDataPoint) {
	require.NotZero(t, point.Timestamp())
	require.NotEqual(t, pmetric.NumberDataPointValueTypeEmpty, point.ValueType())
}

func requireResourceOk(t *testing.T, resource pcommon.Resource) {
	require.NotZero(t, resource.Attributes().Len())
}

func TestWorkingSetMem(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.working_set")
	requireContains(t, metrics, "container.memory.working_set")

	nodeWsMetrics := metrics["k8s.node.memory.working_set"]
	value := nodeWsMetrics[0].Gauge().DataPoints().At(0).IntValue()
	require.Equal(t, int64(1234567890), value)
}

func TestPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.page_faults")
	requireContains(t, metrics, "container.memory.page_faults")

	nodePageFaults := metrics["k8s.node.memory.page_faults"]
	value := nodePageFaults[0].Gauge().DataPoints().At(0).IntValue()
	require.Equal(t, int64(12345), value)
}

func TestMajorPageFaults(t *testing.T) {
	metrics := indexedFakeMetrics()
	requireContains(t, metrics, "k8s.pod.memory.major_page_faults")
	requireContains(t, metrics, "container.memory.major_page_faults")

	nodePageFaults := metrics["k8s.node.memory.major_page_faults"]
	value := nodePageFaults[0].Gauge().DataPoints().At(0).IntValue()
	require.Equal(t, int64(12), value)
}

func TestEmitMetrics(t *testing.T) {
	metrics := indexedFakeMetrics()
	metricNames := []string{
		"k8s.node.network.io",
		"k8s.node.network.errors",
		"k8s.pod.network.io",
		"k8s.pod.network.errors",
	}
	for _, name := range metricNames {
		requireContains(t, metrics, name)
		metric := metrics[name][0]
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			_, found := dp.Attributes().Get("direction")
			require.True(t, found, "expected direction attribute")
		}
	}
}

func requireContains(t *testing.T, metrics map[string][]pmetric.Metric, metricName string) {
	_, found := metrics[metricName]
	require.True(t, found)
}

func indexedFakeMetrics() map[string][]pmetric.Metric {
	mds := fakeMetrics()
	metrics := make(map[string][]pmetric.Metric)
	for _, md := range mds {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
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
	}
	return metrics
}

func fakeMetrics() []pmetric.Metrics {
	rc := &fakeRestClient{}
	statsProvider := NewStatsProvider(rc)
	summary, _ := statsProvider.StatsSummary()
	mgs := map[MetricGroup]bool{
		ContainerMetricGroup: true,
		PodMetricGroup:       true,
		NodeMetricGroup:      true,
	}
	rb := metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder:      metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		PodMetricsBuilder:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
		OtherMetricsBuilder:     metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
	}
	return MetricsData(zap.NewNop(), summary, Metadata{}, mgs, rb, mbs)
}
