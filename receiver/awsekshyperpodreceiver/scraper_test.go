// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsekshyperpodreceiver

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	v1 "k8s.io/api/core/v1"
	"pgregory.net/rapid"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

// mockNodeClient implements k8sclient.NodeClient for testing.
type mockNodeClient struct {
	nodeToLabelsMap map[string]map[k8sclient.Label]int8
}

func (m *mockNodeClient) NodeInfos() map[string]*k8sclient.NodeInfo {
	return nil
}

func (m *mockNodeClient) ClusterFailedNodeCount() int { return 0 }
func (m *mockNodeClient) ClusterNodeCount() int       { return 0 }

func (m *mockNodeClient) NodeToCapacityMap() map[string]v1.ResourceList    { return nil }
func (m *mockNodeClient) NodeToAllocatableMap() map[string]v1.ResourceList { return nil }
func (m *mockNodeClient) NodeToConditionsMap() map[string]map[v1.NodeConditionType]v1.ConditionStatus {
	return nil
}

func (m *mockNodeClient) NodeToLabelsMap() map[string]map[k8sclient.Label]int8 {
	return m.nodeToLabelsMap
}

func newTestScraper(cfg *Config) *scraper {
	settings := receivertest.NewNopSettings(component.MustNewType("awsekshyperpodreceiver"))
	return newScraper(cfg, settings)
}

func defaultTestConfig() *Config {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		ClusterName:      "test-cluster",
	}
	cfg.CollectionInterval = 60 * time.Second
	return cfg
}

// --- isValidHealthStatus tests ---

func TestIsValidHealthStatus(t *testing.T) {
	validStatuses := []int8{0, 1, 2, 3}
	for _, s := range validStatuses {
		assert.True(t, isValidHealthStatus(s), "expected %d to be valid", s)
	}

	invalidStatuses := []int8{-1, 4, 99, -128, 127}
	for _, s := range invalidStatuses {
		assert.False(t, isValidHealthStatus(s), "expected %d to be invalid", s)
	}
}

// --- statusToMetricName tests ---

func TestStatusToMetricName(t *testing.T) {
	tests := []struct {
		status   int8
		expected string
	}{
		{int8(k8sutil.Schedulable), "hyperpod_node_health_status_schedulable"},
		{int8(k8sutil.UnschedulablePendingReplacement), "hyperpod_node_health_status_unschedulable_pending_replacement"},
		{int8(k8sutil.UnschedulablePendingReboot), "hyperpod_node_health_status_unschedulable_pending_reboot"},
		{int8(k8sutil.Unschedulable), "hyperpod_node_health_status_unschedulable"},
	}
	for _, tt := range tests {
		statusStr := k8sutil.HyperPodConditionType(tt.status).String()
		t.Run(statusStr, func(t *testing.T) {
			assert.Equal(t, tt.expected, statusToMetricName[statusStr])
		})
	}
}

// --- Health status metric value tests ---

func TestScrape_SchedulableStatus(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	expected := map[string]int64{
		"hyperpod_node_health_status_schedulable":                       1,
		"hyperpod_node_health_status_unschedulable_pending_replacement": 0,
		"hyperpod_node_health_status_unschedulable_pending_reboot":      0,
		"hyperpod_node_health_status_unschedulable":                     0,
	}
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		dp := m.Gauge().DataPoints().At(0)
		assert.Equal(t, expected[m.Name()], dp.IntValue(), "metric %s", m.Name())
	}
}

func TestScrape_UnschedulableStatus(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Unschedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		dp := m.Gauge().DataPoints().At(0)
		if m.Name() == "hyperpod_node_health_status_unschedulable" {
			assert.Equal(t, int64(1), dp.IntValue())
		} else {
			assert.Equal(t, int64(0), dp.IntValue(), "metric %s should be 0", m.Name())
		}
	}
}

func TestScrape_UnschedulablePendingReplacementStatus(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.UnschedulablePendingReplacement)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		dp := m.Gauge().DataPoints().At(0)
		if m.Name() == "hyperpod_node_health_status_unschedulable_pending_replacement" {
			assert.Equal(t, int64(1), dp.IntValue())
		} else {
			assert.Equal(t, int64(0), dp.IntValue(), "metric %s should be 0", m.Name())
		}
	}
}

func TestScrape_UnschedulablePendingRebootStatus(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.UnschedulablePendingReboot)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		dp := m.Gauge().DataPoints().At(0)
		if m.Name() == "hyperpod_node_health_status_unschedulable_pending_reboot" {
			assert.Equal(t, int64(1), dp.IntValue())
		} else {
			assert.Equal(t, int64(0), dp.IntValue(), "metric %s should be 0", m.Name())
		}
	}
}

// --- Nodes without health label are skipped ---

func TestScrape_NodeWithoutHealthLabel(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		// Empty nodeToLabelsMap — no nodes have labels.
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// No nodes produce data, so no ResourceMetrics should be published.
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

// --- Invalid health status values are skipped ---

func TestScrape_InvalidHealthStatus(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(99)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// Invalid health status produces no metrics, so no ResourceMetrics should be published.
	assert.Equal(t, 0, metrics.ResourceMetrics().Len(), "invalid health status should be skipped")
}

// --- HyperPod prefix stripping tests ---

func TestScrape_HyperPodPrefixRemoval(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"hyperpod-i-1234567890abcdef0": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	dp := sm.Metrics().At(0).Gauge().DataPoints().At(0)
	instanceID, ok := dp.Attributes().Get("instance_id")
	require.True(t, ok)
	assert.Equal(t, "i-1234567890abcdef0", instanceID.Str())
}

func TestScrape_NoPrefixNodeName(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"i-abcdef1234567890": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	dp := sm.Metrics().At(0).Gauge().DataPoints().At(0)
	instanceID, ok := dp.Attributes().Get("instance_id")
	require.True(t, ok)
	assert.Equal(t, "i-abcdef1234567890", instanceID.Str(), "instance_id should equal node name when no hyperpod- prefix")
}

// --- Cluster name attribute tests ---

func TestScrape_ClusterNamePresent(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.ClusterName = "my-cluster"
	s := newTestScraper(cfg)
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	for i := 0; i < sm.Metrics().Len(); i++ {
		dp := sm.Metrics().At(i).Gauge().DataPoints().At(0)
		clusterName, ok := dp.Attributes().Get("cluster_name")
		require.True(t, ok, "cluster_name should be present")
		assert.Equal(t, "my-cluster", clusterName.Str())
	}
}

func TestScrape_ClusterNameEmpty(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.ClusterName = ""
	s := newTestScraper(cfg)
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-1": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	dp := sm.Metrics().At(0).Gauge().DataPoints().At(0)
	_, ok := dp.Attributes().Get("cluster_name")
	assert.False(t, ok, "cluster_name should not be present when config is empty")
}

// --- Empty node list ---

func TestScrape_EmptyNodeList(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// No nodes, so no ResourceMetrics should be published.
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

// --- Shutdown with nil client ---

func TestShutdown_NilClient(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	// k8sClient is nil by default
	assert.Nil(t, s.k8sClient)

	// Should not panic
	err := s.shutdown(t.Context())
	assert.NoError(t, err)
}

// --- Multiple nodes test ---

func TestScrape_MultipleNodes(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"hyperpod-i-111": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
			"hyperpod-i-222": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Unschedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// Always 4 metrics (one per status), each with 2 data points (one per node).
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	assert.Equal(t, 4, sm.Metrics().Len())
	for i := 0; i < sm.Metrics().Len(); i++ {
		assert.Equal(t, 2, sm.Metrics().At(i).Gauge().DataPoints().Len(),
			"metric %s should have 2 data points", sm.Metrics().At(i).Name())
	}
}

// --- Mixed nodes (valid, no label, invalid status) ---

func TestScrape_MixedNodes(t *testing.T) {
	s := newTestScraper(defaultTestConfig())
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"node-valid":          {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
			"node-invalid-status": {k8sclient.SageMakerNodeHealthStatus: int8(99)},
			// node-no-label is not in nodeToLabelsMap
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	// Only node-valid should produce metrics (4 metrics)
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	assert.Equal(t, 4, sm.Metrics().Len())
}

// --- Attributes correctness test ---

func TestScrape_AttributesCorrectness(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.ClusterName = "test-cluster"
	s := newTestScraper(cfg)
	s.nodeClient = &mockNodeClient{
		nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
			"my-node": {k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable)},
		},
	}

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 4, sm.Metrics().Len())

	for i := 0; i < sm.Metrics().Len(); i++ {
		dp := sm.Metrics().At(i).Gauge().DataPoints().At(0)
		attrs := dp.Attributes()

		nodeName, ok := attrs.Get("node_name")
		require.True(t, ok)
		assert.Equal(t, "my-node", nodeName.Str())

		instanceID, ok := attrs.Get("instance_id")
		require.True(t, ok)
		assert.Equal(t, "my-node", instanceID.Str())

		clusterName, ok := attrs.Get("cluster_name")
		require.True(t, ok)
		assert.Equal(t, "test-cluster", clusterName.Str())
	}
}

// --- Property-based test generators ---

// genNodeName generates random node names, some with "hyperpod-" prefix, some without.
func genNodeName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		base := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "base")
		if rapid.Bool().Draw(t, "has_prefix") {
			return "hyperpod-" + base
		}
		return base
	})
}

// genHealthStatus generates a valid health status int8 (0-3).
func genHealthStatus() *rapid.Generator[int8] {
	return rapid.Custom(func(t *rapid.T) int8 {
		return int8(rapid.IntRange(0, 3).Draw(t, "status"))
	})
}

// genClusterName generates empty and non-empty cluster names.
func genClusterName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		if rapid.Bool().Draw(t, "has_cluster") {
			return rapid.StringMatching(`[a-z][a-z0-9\-]{2,15}`).Draw(t, "cluster")
		}
		return ""
	})
}

// --- Property 1: Metric emission correctness ---

func TestProperty_MetricEmissionCorrectness(t *testing.T) {
	ctx := t.Context()
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-10 nodes, each with a valid health status.
		nodeCount := rapid.IntRange(1, 10).Draw(t, "node_count")

		nodeToLabelsMap := make(map[string]map[k8sclient.Label]int8)
		nodeStatuses := make(map[string]int8)

		for i := 0; i < nodeCount; i++ {
			name := genNodeName().Draw(t, "node_name")
			// Ensure unique node names by appending index.
			name = name + "-" + rapid.StringMatching(`[0-9]{4}`).Draw(t, "suffix")
			status := genHealthStatus().Draw(t, "status")

			nodeToLabelsMap[name] = map[k8sclient.Label]int8{
				k8sclient.SageMakerNodeHealthStatus: status,
			}
			nodeStatuses[name] = status
		}

		cfg := defaultTestConfig()
		s := newTestScraper(cfg)
		s.nodeClient = &mockNodeClient{
			nodeToLabelsMap: nodeToLabelsMap,
		}

		metrics, err := s.scrape(ctx)
		if err != nil {
			t.Fatalf("scrape returned error: %v", err)
		}

		sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

		// Should always emit exactly 4 metrics (one per status).
		if sm.Metrics().Len() != 4 {
			t.Fatalf("expected 4 metrics, got %d", sm.Metrics().Len())
		}

		// Each metric should have exactly len(nodeStatuses) data points.
		expectedDPCount := len(nodeStatuses)

		// Group data points by node_name across all metrics.
		type metricEntry struct {
			name  string
			value int64
		}
		nodeMetrics := make(map[string][]metricEntry)
		for i := 0; i < sm.Metrics().Len(); i++ {
			m := sm.Metrics().At(i)
			dps := m.Gauge().DataPoints()
			if dps.Len() != expectedDPCount {
				t.Fatalf("metric %s: expected %d data points, got %d", m.Name(), expectedDPCount, dps.Len())
			}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				nodeName, _ := dp.Attributes().Get("node_name")
				nodeMetrics[nodeName.Str()] = append(nodeMetrics[nodeName.Str()], metricEntry{
					name:  m.Name(),
					value: dp.IntValue(),
				})
			}
		}

		// Verify each node has exactly 4 entries (one per metric), exactly one = 1, rest = 0.
		for nodeName, entries := range nodeMetrics {
			if len(entries) != 4 {
				t.Fatalf("node %s: expected 4 entries, got %d", nodeName, len(entries))
			}

			onesCount := 0
			zerosCount := 0
			for _, e := range entries {
				switch e.value {
				case 1:
					onesCount++
				case 0:
					zerosCount++
				default:
					t.Fatalf("node %s: unexpected metric value %d for %s", nodeName, e.value, e.name)
				}
			}
			if onesCount != 1 {
				t.Fatalf("node %s: expected exactly 1 metric with value 1, got %d", nodeName, onesCount)
			}
			if zerosCount != 3 {
				t.Fatalf("node %s: expected exactly 3 metrics with value 0, got %d", nodeName, zerosCount)
			}

			// Verify the metric set to 1 matches the node's status.
			expectedStatus := k8sutil.HyperPodConditionType(nodeStatuses[nodeName]).String()
			expectedMetricName := statusToMetricName[expectedStatus]
			found := false
			for _, e := range entries {
				if e.value == 1 && e.name == expectedMetricName {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("node %s: expected metric %s to be 1, but it wasn't", nodeName, expectedMetricName)
			}
		}
	})
}

// --- Property 2: Attribute correctness ---

func TestProperty_AttributeCorrectness(t *testing.T) {
	ctx := t.Context()
	rapid.Check(t, func(t *rapid.T) {
		nodeName := genNodeName().Draw(t, "node_name")
		status := genHealthStatus().Draw(t, "status")
		clusterName := genClusterName().Draw(t, "cluster_name")

		cfg := defaultTestConfig()
		cfg.ClusterName = clusterName
		s := newTestScraper(cfg)
		s.nodeClient = &mockNodeClient{
			nodeToLabelsMap: map[string]map[k8sclient.Label]int8{
				nodeName: {k8sclient.SageMakerNodeHealthStatus: status},
			},
		}

		metrics, err := s.scrape(ctx)
		if err != nil {
			t.Fatalf("scrape returned error: %v", err)
		}

		sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
		if sm.Metrics().Len() != 4 {
			t.Fatalf("expected 4 metrics, got %d", sm.Metrics().Len())
		}

		// Expected instance_id: strip "hyperpod-" prefix if present.
		expectedInstanceID := strings.TrimPrefix(nodeName, "hyperpod-")

		for i := 0; i < sm.Metrics().Len(); i++ {
			dp := sm.Metrics().At(i).Gauge().DataPoints().At(0)
			attrs := dp.Attributes()

			// Verify node_name.
			nn, ok := attrs.Get("node_name")
			if !ok || nn.Str() != nodeName {
				t.Fatalf("expected node_name=%q, got %q (ok=%v)", nodeName, nn.Str(), ok)
			}

			// Verify instance_id.
			iid, ok := attrs.Get("instance_id")
			if !ok || iid.Str() != expectedInstanceID {
				t.Fatalf("expected instance_id=%q, got %q (ok=%v)", expectedInstanceID, iid.Str(), ok)
			}

			// Verify cluster_name presence/absence.
			cn, hasCN := attrs.Get("cluster_name")
			if clusterName != "" {
				if !hasCN || cn.Str() != clusterName {
					t.Fatalf("expected cluster_name=%q, got %q (present=%v)", clusterName, cn.Str(), hasCN)
				}
			} else {
				if hasCN {
					t.Fatalf("expected cluster_name to be absent when config is empty, but got %q", cn.Str())
				}
			}
		}
	})
}

// --- Property 3: Any instance type with health label emits metrics ---

func TestProperty_HealthLabelDeterminesEmission(t *testing.T) {
	ctx := t.Context()
	rapid.Check(t, func(t *rapid.T) {
		nodeName := genNodeName().Draw(t, "node_name")
		hasHealthLabel := rapid.Bool().Draw(t, "has_health_label")
		status := genHealthStatus().Draw(t, "status")

		cfg := defaultTestConfig()
		s := newTestScraper(cfg)

		nodeToLabelsMap := make(map[string]map[k8sclient.Label]int8)
		if hasHealthLabel {
			nodeToLabelsMap[nodeName] = map[k8sclient.Label]int8{
				k8sclient.SageMakerNodeHealthStatus: status,
			}
		}

		s.nodeClient = &mockNodeClient{
			nodeToLabelsMap: nodeToLabelsMap,
		}

		metrics, err := s.scrape(ctx)
		if err != nil {
			t.Fatalf("scrape returned error: %v", err)
		}

		hasResourceMetrics := metrics.ResourceMetrics().Len() > 0

		// Metrics emitted iff node has a valid health status label.
		shouldEmit := hasHealthLabel

		if hasResourceMetrics != shouldEmit {
			t.Fatalf("node=%q hasLabel=%v: expected emit=%v, got emit=%v",
				nodeName, hasHealthLabel, shouldEmit, hasResourceMetrics)
		}

		// If metrics were emitted, verify count is exactly 4.
		if hasResourceMetrics {
			sm := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
			if sm.Metrics().Len() != 4 {
				t.Fatalf("expected 4 metrics when emitting, got %d", sm.Metrics().Len())
			}
		}
	})
}
