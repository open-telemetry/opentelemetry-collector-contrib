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
	cfg.Metrics.K8sNodeCPUPressureTotal.Enabled = true
	cfg.Metrics.K8sNodeMemoryPressureAvg.Enabled = true
	cfg.Metrics.K8sNodeMemoryPressureTotal.Enabled = true
	cfg.Metrics.K8sNodeIoPressureAvg.Enabled = true
	cfg.Metrics.K8sNodeIoPressureTotal.Enabled = true
	// Pod PSI
	cfg.Metrics.K8sPodCPUPressureAvg.Enabled = true
	cfg.Metrics.K8sPodCPUPressureTotal.Enabled = true
	cfg.Metrics.K8sPodMemoryPressureAvg.Enabled = true
	cfg.Metrics.K8sPodMemoryPressureTotal.Enabled = true
	cfg.Metrics.K8sPodIoPressureAvg.Enabled = true
	cfg.Metrics.K8sPodIoPressureTotal.Enabled = true
	// Container PSI
	cfg.Metrics.ContainerCPUPressureAvg.Enabled = true
	cfg.Metrics.ContainerCPUPressureTotal.Enabled = true
	cfg.Metrics.ContainerMemoryPressureAvg.Enabled = true
	cfg.Metrics.ContainerMemoryPressureTotal.Enabled = true
	cfg.Metrics.ContainerIoPressureAvg.Enabled = true
	cfg.Metrics.ContainerIoPressureTotal.Enabled = true
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

	// avg metric: 2 pressure types × 3 windows = 6 data points
	requireContains(t, metrics, "k8s.node.cpu.pressure.avg")
	avgMetric := metrics["k8s.node.cpu.pressure.avg"][0]
	assert.Equal(t, 6, avgMetric.Gauge().DataPoints().Len())

	// total metric: 2 pressure types = 2 data points
	requireContains(t, metrics, "k8s.node.cpu.pressure.total")
	totalMetric := metrics["k8s.node.cpu.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())

	// Verify some.total value = 60_000_000_000 ns
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(60_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(5_000_000_000))
}

func TestNodeMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.memory.pressure.avg")
	totalMetric := metrics["k8s.node.memory.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(30_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(1_000_000_000))
}

func TestNodeIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.io.pressure.avg")
	totalMetric := metrics["k8s.node.io.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(10_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(500_000_000))
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
	requireContains(t, metrics, "k8s.pod.cpu.pressure.total")
	assertPSITotalValue(t, metrics["k8s.pod.cpu.pressure.total"][0], metadata.AttributePressureTypeSome, int64(20_000_000_000))
}

func TestPodMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGroup := map[MetricGroup]bool{PodMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.pod.memory.pressure.avg")
	totalMetric := metrics["k8s.pod.memory.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(15_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(500_000_000))
}

func TestPodIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		PodMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	podGroup := map[MetricGroup]bool{PodMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), podSummaryWithPSI(), Metadata{}, podGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.pod.io.pressure.avg")
	totalMetric := metrics["k8s.pod.io.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(5_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(100_000_000))
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
	requireContains(t, metrics, "container.cpu.pressure.total")
	assertPSITotalValue(t, metrics["container.cpu.pressure.total"][0], metadata.AttributePressureTypeSome, int64(8_000_000_000))
}

func TestContainerMemoryPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGroup := map[MetricGroup]bool{ContainerMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "container.memory.pressure.avg")
	totalMetric := metrics["container.memory.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(4_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(200_000_000))
}

func TestContainerIOPressureMetrics(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		ContainerMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	containerGroup := map[MetricGroup]bool{ContainerMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), containerSummaryWithPSI(), Metadata{}, containerGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "container.io.pressure.avg")
	totalMetric := metrics["container.io.pressure.total"][0]
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(2_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(50_000_000))
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
	_, found := metrics["k8s.node.cpu.pressure.total"]
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
	_, found := metrics["k8s.node.io.pressure.total"]
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
	_, found := metrics["k8s.node.io.pressure.total"]
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
		"k8s.node.cpu.pressure.avg", "k8s.node.cpu.pressure.total",
		"k8s.node.memory.pressure.avg", "k8s.node.memory.pressure.total",
		"k8s.node.io.pressure.avg", "k8s.node.io.pressure.total",
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
		"k8s.pod.cpu.pressure.avg", "k8s.pod.cpu.pressure.total",
		"k8s.pod.memory.pressure.avg", "k8s.pod.memory.pressure.total",
		"k8s.pod.io.pressure.avg", "k8s.pod.io.pressure.total",
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
		"container.cpu.pressure.avg", "container.cpu.pressure.total",
		"container.memory.pressure.avg", "container.memory.pressure.total",
		"container.io.pressure.avg", "container.io.pressure.total",
	} {
		_, found := containerMetrics[name]
		require.False(t, found, "PSI metric %q must be disabled by default", name)
	}
}

// TestPSIAvgAttributes verifies the pressure.type and pressure.window attributes are set correctly.
func TestPSIAvgAttributes(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	requireContains(t, metrics, "k8s.node.cpu.pressure.avg")
	avgMetric := metrics["k8s.node.cpu.pressure.avg"][0]

	// Collect all (pressure.type, pressure.window) pairs observed
	type attrPair struct{ ptype, pwindow string }
	seen := make(map[attrPair]float64)
	dps := avgMetric.Gauge().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, _ := dp.Attributes().Get("pressure.type")
		pw, _ := dp.Attributes().Get("pressure.window")
		seen[attrPair{pt.Str(), pw.Str()}] = dp.DoubleValue()
	}

	// 2 types × 3 windows = 6 pairs
	assert.Equal(t, 6, len(seen))
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

// TestPSITotalAttributes verifies the pressure.type attribute on the total counter.
func TestPSITotalAttributes(t *testing.T) {
	cfg := enablePSIConfig()
	mbs := &metadata.MetricsBuilders{
		NodeMetricsBuilder: metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
	}
	nodeGroup := map[MetricGroup]bool{NodeMetricGroup: true}

	metrics := indexedFakeMetrics(MetricsData(newTestLogger(), nodeSummaryWithPSI(), Metadata{}, nodeGroup, nil, mbs, NewCPUUsageCalculator()))

	totalMetric := metrics["k8s.node.cpu.pressure.total"][0]
	dps := totalMetric.Sum().DataPoints()
	assert.Equal(t, 2, dps.Len())

	seen := make(map[string]int64)
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, _ := dp.Attributes().Get("pressure.type")
		seen[pt.Str()] = dp.IntValue()
	}
	assert.Contains(t, seen, "some")
	assert.Contains(t, seen, "full")
	assert.EqualValues(t, 60_000_000_000, seen["some"])
	assert.EqualValues(t, 5_000_000_000, seen["full"])
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

	// Inline pattern: guard the parent, then pass .PSI directly.
	if io != nil {
		addPSIMetrics(mb, metadata.NodeIOPressureMetrics, io.PSI, currentTime)
	}

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

	totalMetric, ok := idx["k8s.node.io.pressure.total"]
	require.True(t, ok, "k8s.node.io.pressure.total must be present")
	assert.Equal(t, 2, totalMetric.Sum().DataPoints().Len())
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeSome, int64(7_000_000_000))
	assertPSITotalValue(t, totalMetric, metadata.AttributePressureTypeFull, int64(300_000_000))
}

// TestAddPSIMetricsNilIOStatsPSI verifies the inlined nil guard: when io is nil,
// addPSIMetrics is never called and no metrics are emitted.
func TestAddPSIMetricsNilIOStatsPSI(t *testing.T) {
	cfg := enablePSIConfig()
	mb := metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type))
	currentTime := pcommon.NewTimestampFromTime(time.Now())

	var io *stats.IOStats // nil — cgroup v1 / Windows

	if io != nil {
		addPSIMetrics(mb, metadata.NodeIOPressureMetrics, io.PSI, currentTime)
	}

	emitted := mb.Emit()
	assert.Equal(t, 0, emitted.ResourceMetrics().Len(), "no metrics should be emitted when io is nil")
}

// --- helpers ---

// newTestLogger returns a no-op zap logger for use in tests.
func newTestLogger() *zap.Logger {
	return zap.NewNop()
}

// assertPSITotalValue finds the data point with the given pressure.type attribute
// and asserts its IntValue matches expected.
func assertPSITotalValue(t *testing.T, metric pmetric.Metric, pressureType metadata.AttributePressureType, expected int64) {
	t.Helper()
	dps := metric.Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		pt, ok := dp.Attributes().Get("pressure.type")
		if !ok {
			continue
		}
		if pt.Str() == pressureType.String() {
			assert.EqualValues(t, expected, dp.IntValue(), "pressure.total for type=%s", pressureType)
			return
		}
	}
	t.Errorf("no data point found for pressure.type=%s", pressureType)
}
