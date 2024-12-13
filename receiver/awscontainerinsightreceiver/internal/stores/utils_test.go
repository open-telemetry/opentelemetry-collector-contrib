// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

type mockCIMetric struct {
	tags   map[string]string
	fields map[string]any
}

func (m *mockCIMetric) GetMetricType() string {
	return "dummy_type"
}

func (m *mockCIMetric) GetFields() map[string]any {
	return m.fields
}

func (m *mockCIMetric) GetTags() map[string]string {
	return m.tags
}

func (m *mockCIMetric) HasField(key string) bool {
	return m.fields[key] != nil
}

func (m *mockCIMetric) AddField(key string, val any) {
	m.fields[key] = val
}

func (m *mockCIMetric) GetField(key string) any {
	return m.fields[key]
}

func (m *mockCIMetric) HasTag(key string) bool {
	return m.tags[key] != ""
}

func (m *mockCIMetric) AddTag(key, val string) {
	m.tags[key] = val
}

func (m *mockCIMetric) GetTag(key string) string {
	return m.tags[key]
}

func (m *mockCIMetric) RemoveTag(key string) {
	delete(m.tags, key)
}

type mockNodeInfoProvider struct{}

func (m *mockNodeInfoProvider) NodeToCapacityMap() map[string]v1.ResourceList {
	return map[string]v1.ResourceList{
		"testNode1": {
			"pods":           *resource.NewQuantity(5, resource.DecimalSI),
			"nvidia.com/gpu": *resource.NewQuantity(20, resource.DecimalExponent),
		},
		"testNode2": {
			"pods": *resource.NewQuantity(10, resource.DecimalSI),
		},
	}
}

func (m *mockNodeInfoProvider) NodeToAllocatableMap() map[string]v1.ResourceList {
	return map[string]v1.ResourceList{
		"testNode1": {
			"pods":           *resource.NewQuantity(15, resource.DecimalSI),
			"nvidia.com/gpu": *resource.NewQuantity(20, resource.DecimalExponent),
		},
		"testNode2": {
			"pods": *resource.NewQuantity(20, resource.DecimalSI),
		},
	}
}

func (m *mockNodeInfoProvider) NodeToLabelsMap() map[string]map[k8sclient.Label]int8 {
	return map[string]map[k8sclient.Label]int8{
		"hyperpod-testNode1": {
			k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable),
		},
		"hyperpod-testNode2": {},
	}
}

func (m *mockNodeInfoProvider) NodeToConditionsMap() map[string]map[v1.NodeConditionType]v1.ConditionStatus {
	return map[string]map[v1.NodeConditionType]v1.ConditionStatus{
		"testNode1": {
			v1.NodeReady:              v1.ConditionTrue,
			v1.NodeDiskPressure:       v1.ConditionFalse,
			v1.NodeMemoryPressure:     v1.ConditionFalse,
			v1.NodePIDPressure:        v1.ConditionFalse,
			v1.NodeNetworkUnavailable: v1.ConditionUnknown,
		},
		"testNode2": {
			v1.NodeReady:          v1.ConditionFalse,
			v1.NodeDiskPressure:   v1.ConditionTrue,
			v1.NodeMemoryPressure: v1.ConditionFalse,
			v1.NodePIDPressure:    v1.ConditionFalse,
			// v1.NodeNetworkUnavailable: v1.ConditionFalse, Commented out intentionally to test missing scenario
		},
	}
}

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	assert.Equal(t, "", parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	assert.Equal(t, "", parseCronJobFromJob("hello-123"))
	assert.Equal(t, "hello", parseCronJobFromJob("hello-1234567890"))
	assert.Equal(t, "", parseCronJobFromJob("hello-123456789a"))
}

func TestUtils_addKubernetesInfo(t *testing.T) {
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	tags := map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.PodNameKey:       "testPod",
		ci.PodIDKey:         "123",
		ci.K8sNamespace:     "testNamespace",
		ci.TypeService:      "testService",
		ci.NodeNameKey:      "testNode",
		ci.Timestamp:        strconv.FormatInt(time.Now().UnixNano(), 10),
	}

	metric := &mockCIMetric{
		tags:   tags,
		fields: fields,
	}

	kubernetesBlob := map[string]any{}
	AddKubernetesInfo(metric, kubernetesBlob, false)
	assert.Equal(t, "", metric.GetTag(ci.ContainerNamekey))
	assert.Equal(t, "", metric.GetTag(ci.PodNameKey))
	assert.Equal(t, "", metric.GetTag(ci.PodIDKey))
	assert.Equal(t, "testNamespace", metric.GetTag(ci.K8sNamespace))
	assert.Equal(t, "testService", metric.GetTag(ci.TypeService))
	assert.Equal(t, "testNode", metric.GetTag(ci.NodeNameKey))

	expectedKubeBlob := map[string]any{"container_name": "testContainer", "host": "testNode", "namespace_name": "testNamespace", "pod_id": "123", "pod_name": "testPod", "service_name": "testService"}
	assert.Equal(t, expectedKubeBlob, kubernetesBlob)
}

func TestUtils_addKubernetesInfoRetainContainerNameTag(t *testing.T) {
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	tags := map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.PodNameKey:       "testPod",
		ci.PodIDKey:         "123",
		ci.K8sNamespace:     "testNamespace",
		ci.TypeService:      "testService",
		ci.NodeNameKey:      "testNode",
		ci.Timestamp:        strconv.FormatInt(time.Now().UnixNano(), 10),
	}

	metric := &mockCIMetric{
		tags:   tags,
		fields: fields,
	}

	kubernetesBlob := map[string]any{}
	AddKubernetesInfo(metric, kubernetesBlob, true)
	assert.Equal(t, "testContainer", metric.GetTag(ci.ContainerNamekey))
	assert.Equal(t, "", metric.GetTag(ci.PodNameKey))
	assert.Equal(t, "", metric.GetTag(ci.PodIDKey))
	assert.Equal(t, "testNamespace", metric.GetTag(ci.K8sNamespace))
	assert.Equal(t, "testService", metric.GetTag(ci.TypeService))
	assert.Equal(t, "testNode", metric.GetTag(ci.NodeNameKey))

	expectedKubeBlob := map[string]any{"container_name": "testContainer", "host": "testNode", "namespace_name": "testNamespace", "pod_id": "123", "pod_name": "testPod", "service_name": "testService"}
	assert.Equal(t, expectedKubeBlob, kubernetesBlob)
}

func TestUtils_TagMetricSource(t *testing.T) {
	types := []string{
		"",
		ci.TypeNode,
		ci.TypeNodeFS,
		ci.TypeNodeNet,
		ci.TypeNodeDiskIO,
		ci.TypePod,
		ci.TypePodNet,
		ci.TypeContainer,
		ci.TypeContainerFS,
		ci.TypeContainerDiskIO,
		ci.TypeInstance,
		ci.TypeInstanceFS,
		ci.TypeInstanceNet,
		ci.TypeInstanceDiskIO,
		ci.TypeContainerGPU,
		ci.TypeContainerNeuron,
	}

	expectedSources := []string{
		"",
		"[\"cadvisor\",\"/proc\",\"pod\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\"]",
		"[\"cadvisor\",\"pod\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\",\"pod\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\"]",
		"[\"cadvisor\",\"/proc\",\"ecsagent\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\",\"calculated\"]",
		"[\"cadvisor\"]",
		"[\"dcgm\",\"pod\",\"calculated\"]",
		"[\"neuron\",\"pod\",\"calculated\"]",
	}
	for i, mtype := range types {
		tags := map[string]string{
			ci.MetricType: mtype,
		}

		metric := &mockCIMetric{
			tags: tags,
		}
		TagMetricSource(metric)
		assert.Equal(t, expectedSources[i], metric.tags[ci.SourcesKey])
	}
}

func TestUtils_TagMetricSourceWindows(t *testing.T) {
	types := []string{
		"",
		ci.TypeNode,
		ci.TypeNodeFS,
		ci.TypeNodeNet,
		ci.TypeNodeDiskIO,
		ci.TypePod,
		ci.TypePodNet,
		ci.TypeContainer,
		ci.TypeContainerFS,
		ci.TypeContainerDiskIO,
	}

	expectedSources := []string{
		"",
		"[\"kubelet\",\"pod\",\"calculated\"]",
		"[\"kubelet\",\"calculated\"]",
		"[\"kubelet\",\"calculated\"]",
		"[\"kubelet\"]",
		"[\"kubelet\",\"pod\",\"calculated\"]",
		"[\"kubelet\",\"calculated\"]",
		"[\"kubelet\",\"pod\",\"calculated\"]",
		"[\"kubelet\",\"calculated\"]",
		"[\"kubelet\"]",
	}
	for i, mtype := range types {
		tags := map[string]string{
			ci.MetricType:      mtype,
			ci.OperatingSystem: ci.OperatingSystemWindows,
		}

		metric := &mockCIMetric{
			tags: tags,
		}
		TagMetricSource(metric)
		assert.Equal(t, expectedSources[i], metric.tags[ci.SourcesKey])
	}
}
