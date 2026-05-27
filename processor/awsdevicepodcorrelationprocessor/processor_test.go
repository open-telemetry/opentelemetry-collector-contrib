// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsdevicepodcorrelationprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor/internal/kubelet"
)

// mockLookup implements deviceLookup for testing.
type mockLookup struct {
	data map[string]map[string]*kubelet.ContainerInfo
}

func newMockLookup(data map[string]map[string]*kubelet.ContainerInfo) *mockLookup {
	return &mockLookup{data: data}
}

func (m *mockLookup) GetContainerInfo(deviceID, resourceName string) *kubelet.ContainerInfo {
	if rn, ok := m.data[deviceID]; ok {
		return rn[resourceName]
	}
	return nil
}

// newTestProcessor creates a processor with a mock lookup for testing.
func newTestProcessor(cfg *Config, lookup deviceLookup) *devicePodCorrelationProcessor {
	return &devicePodCorrelationProcessor{config: cfg, logger: zap.NewNop(), lookup: lookup}
}

func newTestMetrics(deviceIDKey, deviceIDVal string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr(deviceIDKey, deviceIDVal)
	return md
}

func newTestMetricsWithResourceAttr(resourceKey, resourceVal string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr(resourceKey, resourceVal)
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_metric")
	m.SetEmptyGauge().DataPoints().AppendEmpty()
	return md
}

func TestProcessMetrics_CorrelatesDeviceToPod(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"aws.amazon.com/neurondevice": {PodName: "ml-pod", Namespace: "default", ContainerName: "trainer"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"aws.amazon.com/neurondevice"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("NeuronDevice", "0")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	podVal, ok := dp.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "ml-pod", podVal.AsString())
	nsVal, _ := dp.Attributes().Get(k8sNamespaceKey)
	assert.Equal(t, "default", nsVal.AsString())
	cVal, _ := dp.Attributes().Get(containerNameKey)
	assert.Equal(t, "trainer", cVal.AsString())
}

func TestProcessMetrics_NoMatchLeavesDatapointUnchanged(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"aws.amazon.com/neurondevice"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("NeuronDevice", "99")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	_, ok := dp.Attributes().Get(k8sPodNameKey)
	assert.False(t, ok)
}

func TestProcessMetrics_SkipsAlreadyEnrichedDatapoints(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"aws.amazon.com/neurondevice": {PodName: "should-not-overwrite", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"aws.amazon.com/neurondevice"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("NeuronDevice", "0")
	md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().PutStr(k8sPodNameKey, "existing-pod")

	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	podVal, _ := dp.Attributes().Get(k8sPodNameKey)
	assert.Equal(t, "existing-pod", podVal.AsString())
}

func TestProcessMetrics_ResourceLevelDeviceID(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"efa3": {"vpc.amazonaws.com/efa": {PodName: "efa-pod", Namespace: "prod", ContainerName: "worker"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "efa", DeviceIDAttribute: "device", DeviceIDSource: DeviceIDSourceResource, ResourceNames: []string{"vpc.amazonaws.com/efa"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetricsWithResourceAttr("device", "efa3")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	podVal, ok := dp.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "efa-pod", podVal.AsString())
}

func TestProcessMetrics_SumMetricType(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"aws.amazon.com/neurondevice": {PodName: "sum-pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"aws.amazon.com/neurondevice"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("counter")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("NeuronDevice", "0")

	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dpOut := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
	podVal, ok := dpOut.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "sum-pod", podVal.AsString())
}

func TestProcessMetrics_FallbackResourceNames(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"aws.amazon.com/neuron": {PodName: "fallback-pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "NeuronDevice", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"aws.amazon.com/neurondevice", "aws.amazon.com/neuron"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("NeuronDevice", "0")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	podVal, ok := dp.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "fallback-pod", podVal.AsString())
}

func TestProcessMetrics_HistogramMetricType(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"res": {PodName: "hist-pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("latency")
	dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("dev", "0")

	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dpOut := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
	podVal, ok := dpOut.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "hist-pod", podVal.AsString())
}

func TestShutdown_NilClient(t *testing.T) {
	p := &devicePodCorrelationProcessor{}
	assert.NoError(t, p.Shutdown(t.Context()))
}

func TestStart_CreatesClientAndConnects(t *testing.T) {
	cfg := &Config{
		KubeletSocketPath: "/nonexistent/socket",
		DeviceTypes: []DeviceTypeConfig{
			{Name: "neuron", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res.a", "res.b"}},
			{Name: "efa", DeviceIDAttribute: "dev2", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res.b", "res.c"}},
		},
	}
	p := newProcessor(cfg, zap.NewNop())

	// grpc.NewClient uses lazy connection, so Start() succeeds even with a
	// nonexistent socket. The actual connection is attempted on the first
	// List() call in refresh(). This test verifies the client is created
	// and resource names are registered.
	err := p.Start(t.Context(), nil)
	require.NoError(t, err)
	assert.NotNil(t, p.client)
	assert.NotNil(t, p.lookup)
	assert.NoError(t, p.Shutdown(t.Context()))
}

func TestProcessMetrics_NoDeviceIDAttribute(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"res": {PodName: "pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "missing_attr", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("other_attr", "0")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	_, ok := dp.Attributes().Get(k8sPodNameKey)
	assert.False(t, ok)
}

func TestProcessMetrics_FirstMatchingDeviceTypeWins(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {
			"res.a": {PodName: "pod-a", Namespace: "ns", ContainerName: "c"},
			"res.b": {PodName: "pod-b", Namespace: "ns", ContainerName: "c"},
		},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "type-a", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res.a"}},
			{Name: "type-b", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res.b"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := newTestMetrics("dev", "0")
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dp := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	podVal, _ := dp.Attributes().Get(k8sPodNameKey)
	assert.Equal(t, "pod-a", podVal.AsString(), "first matching device type should win")
}

func TestProcessMetrics_EmptyMetrics(t *testing.T) {
	lookup := newMockLookup(nil)
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := pmetric.NewMetrics()
	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ResourceMetrics().Len())
}

func TestProcessMetrics_ExponentialHistogramMetricType(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"res": {PodName: "exphist-pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("exp_latency")
	dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("dev", "0")

	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dpOut := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0)
	podVal, ok := dpOut.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "exphist-pod", podVal.AsString())
}

func TestProcessMetrics_SummaryMetricType(t *testing.T) {
	lookup := newMockLookup(map[string]map[string]*kubelet.ContainerInfo{
		"0": {"res": {PodName: "summary-pod", Namespace: "ns", ContainerName: "c"}},
	})
	cfg := &Config{
		DeviceTypes: []DeviceTypeConfig{
			{Name: "gpu", DeviceIDAttribute: "dev", DeviceIDSource: DeviceIDSourceDatapoint, ResourceNames: []string{"res"}},
		},
	}
	p := newTestProcessor(cfg, lookup)
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("request_duration")
	dp := m.SetEmptySummary().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("dev", "0")

	result, err := p.processMetrics(t.Context(), md)
	require.NoError(t, err)

	dpOut := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0)
	podVal, ok := dpOut.Attributes().Get(k8sPodNameKey)
	assert.True(t, ok)
	assert.Equal(t, "summary-pod", podVal.AsString())
}
