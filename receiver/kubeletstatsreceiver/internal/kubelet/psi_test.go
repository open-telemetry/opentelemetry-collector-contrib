// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

// psiData builds a PSIData with realistic values.
func psiData(totalNs uint64, avg10, avg60, avg300 float64) stats.PSIData {
	return stats.PSIData{
		Total:  totalNs,
		Avg10:  avg10,
		Avg60:  avg60,
		Avg300: avg300,
	}
}

// psiStats builds a *PSIStats with some and full data.
func psiStats(someTotal, fullTotal uint64) *stats.PSIStats {
	return &stats.PSIStats{
		Some: psiData(someTotal, 10.5, 8.2, 5.1),
		Full: psiData(fullTotal, 2.3, 1.8, 0.9),
	}
}

// nodeSummaryWithPSI builds a minimal stats.Summary for a single node with all PSI fields set.
func nodeSummaryWithPSI() *stats.Summary {
	now := time.Now()
	return &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
			CPU: &stats.CPUStats{
				Time: v1.Time{Time: now},
				PSI:  psiStats(60_000_000_000, 5_000_000_000),
			},
			Memory: &stats.MemoryStats{
				Time: v1.Time{Time: now},
				PSI:  psiStats(30_000_000_000, 1_000_000_000),
			},
			IO: &stats.IOStats{
				Time: v1.Time{Time: now},
				PSI:  psiStats(10_000_000_000, 500_000_000),
			},
		},
	}
}

// podSummaryWithPSI builds a summary with one pod that has all PSI fields set.
func podSummaryWithPSI() *stats.Summary {
	now := time.Now()
	return &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
		},
		Pods: []stats.PodStats{
			{
				PodRef:    stats.PodReference{Name: "mypod", Namespace: "default", UID: "uid-1"},
				StartTime: v1.Time{Time: now.Add(-30 * time.Minute)},
				CPU: &stats.CPUStats{
					Time: v1.Time{Time: now},
					PSI:  psiStats(20_000_000_000, 2_000_000_000),
				},
				Memory: &stats.MemoryStats{
					Time: v1.Time{Time: now},
					PSI:  psiStats(15_000_000_000, 500_000_000),
				},
				IO: &stats.IOStats{
					Time: v1.Time{Time: now},
					PSI:  psiStats(5_000_000_000, 100_000_000),
				},
			},
		},
	}
}

// containerSummaryWithPSI builds a summary with one pod and one container that has PSI fields.
func containerSummaryWithPSI() *stats.Summary {
	now := time.Now()
	return &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
		},
		Pods: []stats.PodStats{
			{
				PodRef:    stats.PodReference{Name: "mypod", Namespace: "default", UID: "uid-2"},
				StartTime: v1.Time{Time: now.Add(-30 * time.Minute)},
				Containers: []stats.ContainerStats{
					{
						Name:      "mycontainer",
						StartTime: v1.Time{Time: now.Add(-20 * time.Minute)},
						CPU: &stats.CPUStats{
							Time: v1.Time{Time: now},
							PSI:  psiStats(8_000_000_000, 800_000_000),
						},
						Memory: &stats.MemoryStats{
							Time: v1.Time{Time: now},
							PSI:  psiStats(4_000_000_000, 200_000_000),
						},
						IO: &stats.IOStats{
							Time: v1.Time{Time: now},
							PSI:  psiStats(2_000_000_000, 50_000_000),
						},
					},
				},
			},
		},
	}
}

// enablePSIConfig returns a MetricsBuilderConfig with all 18 PSI metrics enabled.
func enablePSIConfig() metadata.MetricsBuilderConfig {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	// Node PSI
	cfg.Metrics.K8sNodeCPUPressureAvg.Enabled = true
	cfg.Metrics.K8sNodeCPUPressureTime.Enabled = true
	cfg.Metrics.K8sNodeMemoryPressureAvg.Enabled = true
	cfg.Metrics.K8sNodeMemoryPressureTime.Enabled = true
	cfg.Metrics.K8sNodeIoPressureAvg.Enabled = true
	cfg.Metrics.K8sNodeIoPressureTime.Enabled = true
	// Pod PSI
	cfg.Metrics.K8sPodCPUPressureAvg.Enabled = true
	cfg.Metrics.K8sPodCPUPressureTime.Enabled = true
	cfg.Metrics.K8sPodMemoryPressureAvg.Enabled = true
	cfg.Metrics.K8sPodMemoryPressureTime.Enabled = true
	cfg.Metrics.K8sPodIoPressureAvg.Enabled = true
	cfg.Metrics.K8sPodIoPressureTime.Enabled = true
	// Container PSI
	cfg.Metrics.ContainerCPUPressureAvg.Enabled = true
	cfg.Metrics.ContainerCPUPressureTime.Enabled = true
	cfg.Metrics.ContainerMemoryPressureAvg.Enabled = true
	cfg.Metrics.ContainerMemoryPressureTime.Enabled = true
	cfg.Metrics.ContainerIoPressureAvg.Enabled = true
	cfg.Metrics.ContainerIoPressureTime.Enabled = true
	return cfg
}

// --- Node PSI tests ---

func TestNodeCPUPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	// avg metric: 2 psi types × 3 windows = 6 data points
	requireContains(t, metrics, "k8s.node.cpu.pressure.avg")
	avgMetric := metrics["k8s.node.cpu.pressure.avg"][0]
	assert.Equal(t, 6, avgMetric.Gauge().DataPoints().Len())

	// time metric: 2 psi types = 2 data points
	requireContains(t, metrics, "k8s.node.cpu.pressure.time")
	timeMetric := metrics["k8s.node.cpu.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())

	// Verify some.time value = 60_000_000_000 ns → 60.0 s
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 60.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 5.0)
}

func TestNodeMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.memory.pressure.avg")
	timeMetric := metrics["k8s.node.memory.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 30.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 1.0)
}

func TestNodeIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.io.pressure.avg")
	timeMetric := metrics["k8s.node.io.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 10.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.5)
}

// --- Pod PSI tests ---

func TestPodCPUPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGroup := map[MetricGroup]bool{PodMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.pod.cpu.pressure.avg")
	requireContains(t, metrics, "k8s.pod.cpu.pressure.time")
	assertPSITimeValue(t, metrics["k8s.pod.cpu.pressure.time"][0], metadata.AttributePsiTypeSome, 20.0)
}

func TestPodMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGroup := map[MetricGroup]bool{PodMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.pod.memory.pressure.avg")
	timeMetric := metrics["k8s.pod.memory.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 15.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.5)
}

func TestPodIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGroup := map[MetricGroup]bool{PodMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.pod.io.pressure.avg")
	timeMetric := metrics["k8s.pod.io.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 5.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.1)
}

// --- Container PSI tests ---

func TestContainerCPUPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGroup := map[MetricGroup]bool{ContainerMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "container.cpu.pressure.avg")
	requireContains(t, metrics, "container.cpu.pressure.time")
	assertPSITimeValue(t, metrics["container.cpu.pressure.time"][0], metadata.AttributePsiTypeSome, 8.0)
}

func TestContainerMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGroup := map[MetricGroup]bool{ContainerMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "container.memory.pressure.avg")
	timeMetric := metrics["container.memory.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 4.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.2)
}

func TestContainerIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGroup := map[MetricGroup]bool{ContainerMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "container.io.pressure.avg")
	timeMetric := metrics["container.io.pressure.time"][0]
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 2.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.05)
}

// --- Nil-safety tests ---

// TestPSINilCPU verifies that a node with nil CPU.PSI emits no PSI metrics.
func TestPSINilCPU(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	now := time.Now()
	summary := &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
			// CPU.PSI deliberately nil
			CPU: &stats.CPUStats{
				Time: v1.Time{Time: now},
				// PSI is nil
			},
		},
	}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), summary, Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))
	_, found := metrics["k8s.node.cpu.pressure.time"]
	require.False(t, found, "no CPU pressure metrics should be emitted when PSI is nil")
}

// TestPSINilIO verifies that a node with nil IO field emits no IO PSI metrics.
func TestPSINilIO(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	now := time.Now()
	summary := &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
			// IO is nil (cgroup v1 node)
		},
	}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), summary, Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))
	_, found := metrics["k8s.node.io.pressure.time"]
	require.False(t, found, "no IO pressure metrics should be emitted when IO is nil")
}

// TestPSIIONilPSIField verifies that a node where IOStats exists but IOStats.PSI is nil
// emits no IO PSI metrics. This is distinct from IO == nil (cgroup v1 vs partially-populated struct).
func TestPSIIONilPSIField(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	now := time.Now()
	summary := &stats.Summary{
		Node: stats.NodeStats{
			NodeName:  "worker-1",
			StartTime: v1.Time{Time: now.Add(-time.Hour)},
			IO: &stats.IOStats{
				Time: v1.Time{Time: now},
				// PSI intentionally omitted — IOStats exists but carries no PSI data.
			},
		},
	}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), summary, Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))
	_, found := metrics["k8s.node.io.pressure.time"]
	require.False(t, found, "no IO pressure metrics should be emitted when IOStats.PSI is nil")
}

// TestPSIDisabledByDefault verifies that PSI metrics are NOT emitted with the default config
// for any of the 9 scope×resource combinations (node/pod/container × CPU/memory/IO).
func TestPSIDisabledByDefault(t *testing.T) {
	defaultCfg := metadata.NewDefaultMetricsBuilderConfig()

	// Node scope
	nodeMBS := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(defaultCfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGrp := map[MetricGroup]bool{NodeMetricGroup: true}
	nodeMetrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGrp, nil, nodeMBS, NewCPUUsageCalculator()))

	for _, name := range []string{
		"k8s.node.cpu.pressure.avg", "k8s.node.cpu.pressure.time",
		"k8s.node.memory.pressure.avg", "k8s.node.memory.pressure.time",
		"k8s.node.io.pressure.avg", "k8s.node.io.pressure.time",
	} {
		_, found := nodeMetrics[name]
		require.False(t, found, "PSI metric %q must be disabled by default", name)
	}

	// Pod scope
	podMBS := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(defaultCfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGrp := map[MetricGroup]bool{PodMetricGroup: true}
	podMetrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGrp, nil, podMBS, NewCPUUsageCalculator()))

	for _, name := range []string{
		"k8s.pod.cpu.pressure.avg", "k8s.pod.cpu.pressure.time",
		"k8s.pod.memory.pressure.avg", "k8s.pod.memory.pressure.time",
		"k8s.pod.io.pressure.avg", "k8s.pod.io.pressure.time",
	} {
		_, found := podMetrics[name]
		require.False(t, found, "PSI metric %q must be disabled by default", name)
	}

	// Container scope
	containerMBS := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(defaultCfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGrp := map[MetricGroup]bool{ContainerMetricGroup: true}
	containerMetrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGrp, nil, containerMBS, NewCPUUsageCalculator()))

	for _, name := range []string{
		"container.cpu.pressure.avg", "container.cpu.pressure.time",
		"container.memory.pressure.avg", "container.memory.pressure.time",
		"container.io.pressure.avg", "container.io.pressure.time",
	} {
		_, found := containerMetrics[name]
		require.False(t, found, "PSI metric %q must be disabled by default", name)
	}
}

// TestPSIAvgAttributes verifies the psi.type and psi.window attributes are set correctly.
func TestPSIAvgAttributes(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.cpu.pressure.avg")
	avgMetric := metrics["k8s.node.cpu.pressure.avg"][0]

	// Collect all (psi.type, psi.window) pairs observed
	type attrPair struct{ ptype, pwindow string }
	seen := make(map[attrPair]float64)
	dps := avgMetric.Gauge().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, _ := dp.Attributes().Get("psi.type")
		pw, _ := dp.Attributes().Get("psi.window")
		seen[attrPair{pt.Str(), pw.Str()}] = dp.DoubleValue()
	}

	// 2 types × 3 windows = 6 pairs
	assert.Len(t, seen, 6)
	assert.Contains(t, seen, attrPair{"some", "10s"})
	assert.Contains(t, seen, attrPair{"some", "60s"})
	assert.Contains(t, seen, attrPair{"some", "300s"})
	assert.Contains(t, seen, attrPair{"full", "10s"})
	assert.Contains(t, seen, attrPair{"full", "60s"})
	assert.Contains(t, seen, attrPair{"full", "300s"})

	// Spot-check some.10s = 10.5 (from psiStats helper)
	assert.InDelta(t, 10.5, seen[attrPair{"some", "10s"}], 1e-9)
	// Spot-check full.10s = 2.3
	assert.InDelta(t, 2.3, seen[attrPair{"full", "10s"}], 1e-9)
}

// TestPSITimeAttributes verifies the psi.type attribute on the time counter.
func TestPSITimeAttributes(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	timeMetric := metrics["k8s.node.cpu.pressure.time"][0]
	dps := timeMetric.Sum().DataPoints()
	assert.Equal(t, 2, dps.Len())

	seen := make(map[string]float64)
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, _ := dp.Attributes().Get("psi.type")
		seen[pt.Str()] = dp.DoubleValue()
	}
	assert.Contains(t, seen, "some")
	assert.Contains(t, seen, "full")
	assert.InDelta(t, 60.0, seen["some"], 1e-9)
	assert.InDelta(t, 5.0, seen["full"], 1e-9)
}

// --- addPSIMetrics direct unit tests ---

// TestAddPSIMetricsWithIOStatsPSI verifies that calling addPSIMetrics directly
// with an IOStats.PSI value (the inlined call pattern) emits the correct data points.
// This test must remain GREEN after the addIOPSIMetrics wrapper is removed.
func TestAddPSIMetricsWithIOStatsPSI(t *testing.T) {
	cfg := enablePSIConfig()
	mb := metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type))
	currentTime := pcommon.NewTimestampFromTime(time.Now())

	io := &stats.IOStats{
		PSI: psiStats(7_000_000_000, 300_000_000),
	}

	// Pass io.PSI directly; io is guaranteed non-nil above.
	addPSIMetrics(mb, metadata.NodeIOPressureMetrics, io.PSI, currentTime)

	emitted := mb.Emit()
	require.Equal(t, 1, emitted.ResourceMetrics().Len())
	sm := emitted.ResourceMetrics().At(0).ScopeMetrics()
	require.Equal(t, 1, sm.Len())
	ms := sm.At(0).Metrics()

	idx := make(map[string]pmetric.Metric, ms.Len())
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		idx[m.Name()] = m
	}

	timeMetric, ok := idx["k8s.node.io.pressure.time"]
	require.True(t, ok, "k8s.node.io.pressure.time must be present")
	assert.Equal(t, 2, timeMetric.Sum().DataPoints().Len())
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeSome, 7.0)
	assertPSITimeValue(t, timeMetric, metadata.AttributePsiTypeFull, 0.3)
}

// TestAddPSIMetricsNilIOStatsPSI verifies the inlined nil guard: when io is nil,
// addPSIMetrics is never called and no metrics are emitted.
func TestAddPSIMetricsNilIOStatsPSI(t *testing.T) {
	cfg := enablePSIConfig()
	mb := metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type))
	currentTime := pcommon.NewTimestampFromTime(time.Now())

	// Simulate the nil case: pass nil PSIStats directly.
	// addPSIMetrics is nil-safe and must not emit any metrics when s == nil.
	addPSIMetrics(mb, metadata.NodeIOPressureMetrics, nil, currentTime)

	emitted := mb.Emit()
	assert.Equal(t, 0, emitted.ResourceMetrics().Len(), "no metrics should be emitted when io is nil")
}

// --- helpers ---

// newTestLogger returns a no-op zap logger for use in tests.
func newTestLogger() *zap.Logger {
	return zap.NewNop()
}

// assertPSITimeValue finds the data point with the given psi.type attribute
// and asserts its DoubleValue matches expected (in seconds).
func assertPSITimeValue(t *testing.T, metric pmetric.Metric, psiType metadata.AttributePsiType, expected float64) {
	t.Helper()
	dps := metric.Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, ok := dp.Attributes().Get("psi.type")
		if !ok {
			continue
		}
		if pt.Str() == psiType.String() {
			assert.InDelta(t, expected, dp.DoubleValue(), 1e-9, "pressure.time for type=%s", psiType)
			return
		}
	}
	t.Errorf("no data point found for psi.type=%s", psiType)
}
