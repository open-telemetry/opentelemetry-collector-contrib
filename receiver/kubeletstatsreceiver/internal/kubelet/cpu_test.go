// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

// nodeSummary builds a minimal stats.Summary for a single node with the given cumulative
// CPU time (in nanocore-seconds) sampled at the given time.
func nodeSummary(usageCoreNanoSeconds uint64, sampleTime time.Time) *stats.Summary {
	return &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-42",
			StartTime: v1.Time{Time: sampleTime.Add(-time.Hour)},
			CPU: &stats.CPUStats{
				Time:                 v1.Time{Time: sampleTime},
				UsageCoreNanoSeconds: &usageCoreNanoSeconds,
				// UsageNanoCores is deliberately left unset: with the feature gate
				// enabled it must not be consulted at all.
			},
		},
	}
}

func TestScrapeBasedCPUUsage(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverKubeletstatsCPUUsageScrapeBasedFeatureGate, true)()

	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}
	calc := NewCPUUsageCalculator()

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// First scrape: no previous sample, so no usage should be recorded, but cpu.time
	// (the cumulative counter) is always recorded regardless of the gate.
	firstMetrics := indexedFakeMetrics(MetricsData(zap.NewNop(), nodeSummary(10_000_000_000, base), Metadata{}, nodeGroup, nil, mbs, calc))
	_, found := firstMetrics["k8s.node.cpu.usage"]
	require.False(t, found)
	requireContains(t, firstMetrics, "k8s.node.cpu.time")
	require.InDelta(t, 10.0, firstMetrics["k8s.node.cpu.time"][0].Sum().DataPoints().At(0).DoubleValue(), 1e-9)

	// Second scrape, 2s later: cumulative CPU time advanced by 4 CPU-seconds ->
	// (14-10)/2 = 2 cores of usage.
	secondMetrics := indexedFakeMetrics(MetricsData(zap.NewNop(), nodeSummary(14_000_000_000, base.Add(2*time.Second)), Metadata{}, nodeGroup, nil, mbs, calc))
	usageMetrics, found := secondMetrics["k8s.node.cpu.usage"]
	require.True(t, found)
	require.InDelta(t, 2.0, usageMetrics[0].Gauge().DataPoints().At(0).DoubleValue(), 1e-9)
}

func TestScrapeBasedCPUUsage_DisabledGateUsesUsageNanoCores(t *testing.T) {
	// The gate is beta (on by default); disable it explicitly to test the legacy UsageNanoCores path.
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverKubeletstatsCPUUsageScrapeBasedFeatureGate, false)()

	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}
	calc := NewCPUUsageCalculator()

	usageNanoCores := uint64(1_500_000_000) // 1.5 cores
	usageCoreNanoSeconds := uint64(10_000_000_000)
	summary := &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-42",
			StartTime: v1.Time{Time: time.Now().Add(-time.Hour)},
			CPU: &stats.CPUStats{
				Time:                 v1.Time{Time: time.Now()},
				UsageNanoCores:       &usageNanoCores,
				UsageCoreNanoSeconds: &usageCoreNanoSeconds,
			},
		},
	}

	metrics := indexedFakeMetrics(MetricsData(zap.NewNop(), summary, Metadata{}, nodeGroup, nil, mbs, calc))
	usageMetrics, found := metrics["k8s.node.cpu.usage"]
	require.True(t, found)
	require.InDelta(t, 1.5, usageMetrics[0].Gauge().DataPoints().At(0).DoubleValue(), 1e-9)
}

func TestRoundedUtilization(t *testing.T) {
	tests := []struct {
		name       string
		usageCores float64
		limitCores float64
		expected   float64
	}{
		{
			// Plain float64 division yields 2.9999999999999996.
			name:       "removes float64 artifact",
			usageCores: float64(300_000_000) / nanoCoresPerCore,
			limitCores: 0.1,
			expected:   3,
		},
		{
			name:       "rounds to significant digits of the inputs",
			usageCores: 1,
			limitCores: 3,
			expected:   0.3333333333,
		},
		{
			name:       "fractional millicore limit from node capacity",
			usageCores: float64(123_456_789) / nanoCoresPerCore,
			limitCores: float64(1500) / milliCoresPerCore,
			expected:   0.082304526,
		},
		{
			name:       "zero usage",
			usageCores: 0,
			limitCores: 2,
			expected:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, roundedUtilization(tt.usageCores, tt.limitCores))
		})
	}
}

func TestCPUUtilizationRoundingFeatureGate(t *testing.T) {
	newPodMetrics := func() pmetric.Metrics {
		cfg := metadata.NewDefaultMetricsBuilderConfig()
		cfg.Metrics.K8sPodCPUNodeUtilization.Enabled = true
		cfg.Metrics.K8sPodCPULimitUtilization.Enabled = true
		cfg.Metrics.K8sPodCPURequestUtilization.Enabled = true
		mb := metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type))
		// 0.3 cores used against a 100m limit/request and a 0.1 core node: the float64
		// quotient is 2.9999999999999996, the rounded one is 3.
		r := resources{cpuLimit: 0.1, cpuRequest: 0.1}
		addCPUUtilizationMetrics(mb, metadata.PodCPUMetrics, float64(300_000_000)/nanoCoresPerCore, pcommon.Timestamp(0), r, 0.1)
		return mb.Emit()
	}
	utilizationValues := func(m pmetric.Metrics) map[string]float64 {
		out := map[string]float64{}
		ms := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < ms.Len(); i++ {
			out[ms.At(i).Name()] = ms.At(i).Gauge().DataPoints().At(0).DoubleValue()
		}
		return out
	}
	names := []string{"k8s.pod.cpu.node.utilization", "k8s.pod.cpu_limit_utilization", "k8s.pod.cpu_request_utilization"}

	t.Run("disabled", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverKubeletStatsCPUUtilizationRoundingFeatureGate, false)()
		values := utilizationValues(newPodMetrics())
		for _, name := range names {
			require.Contains(t, values, name)
			require.Equal(t, 2.9999999999999996, values[name], name)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverKubeletStatsCPUUtilizationRoundingFeatureGate, true)()
		values := utilizationValues(newPodMetrics())
		for _, name := range names {
			require.Contains(t, values, name)
			require.Equal(t, 3.0, values[name], name)
		}
	})
}
