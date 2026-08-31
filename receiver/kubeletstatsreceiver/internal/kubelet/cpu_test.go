// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	// Gate defaults to disabled; explicitly assert the default (legacy) behavior is
	// unaffected by the new code path.
	require.False(t, metadata.ReceiverKubeletstatsCPUUsageScrapeBasedFeatureGate.IsEnabled())

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
