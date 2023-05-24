// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stores

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type mockCIMetric struct {
	tags   map[string]string
	fields map[string]interface{}
}

func (m *mockCIMetric) HasField(key string) bool {
	return m.fields[key] != nil
}

func (m *mockCIMetric) AddField(key string, val interface{}) {
	m.fields[key] = val
}

func (m *mockCIMetric) GetField(key string) interface{} {
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

type mockNodeInfoProvider struct {
}

func (m *mockNodeInfoProvider) NodeToCapacityMap() map[string]v1.ResourceList {
	return map[string]v1.ResourceList{
		"testNode1": {
			"pods": *resource.NewQuantity(5, resource.DecimalSI),
		},
		"testNode2": {
			"pods": *resource.NewQuantity(10, resource.DecimalSI),
		},
	}
}

func (m *mockNodeInfoProvider) NodeToAllocatableMap() map[string]v1.ResourceList {
	return map[string]v1.ResourceList{
		"testNode1": {
			"pods": *resource.NewQuantity(15, resource.DecimalSI),
		},
		"testNode2": {
			"pods": *resource.NewQuantity(20, resource.DecimalSI),
		},
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
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	tags := map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.K8sPodNameKey:    "testPod",
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

	kubernetesBlob := map[string]interface{}{}
	AddKubernetesInfo(metric, kubernetesBlob, false)
	assert.Equal(t, "", metric.GetTag(ci.ContainerNamekey))
	assert.Equal(t, "", metric.GetTag(ci.K8sPodNameKey))
	assert.Equal(t, "", metric.GetTag(ci.PodIDKey))
	assert.Equal(t, "testNamespace", metric.GetTag(ci.K8sNamespace))
	assert.Equal(t, "testService", metric.GetTag(ci.TypeService))
	assert.Equal(t, "testNode", metric.GetTag(ci.NodeNameKey))

	expectedKubeBlob := map[string]interface{}{"container_name": "testContainer", "host": "testNode", "namespace_name": "testNamespace", "pod_id": "123", "pod_name": "testPod", "service_name": "testService"}
	assert.Equal(t, expectedKubeBlob, kubernetesBlob)
}

func TestUtils_addKubernetesInfoRetainContainerNameTag(t *testing.T) {
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	tags := map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.K8sPodNameKey:    "testPod",
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

	kubernetesBlob := map[string]interface{}{}
	AddKubernetesInfo(metric, kubernetesBlob, true)
	assert.Equal(t, "testContainer", metric.GetTag(ci.ContainerNamekey))
	assert.Equal(t, "", metric.GetTag(ci.K8sPodNameKey))
	assert.Equal(t, "", metric.GetTag(ci.PodIDKey))
	assert.Equal(t, "testNamespace", metric.GetTag(ci.K8sNamespace))
	assert.Equal(t, "testService", metric.GetTag(ci.TypeService))
	assert.Equal(t, "testNode", metric.GetTag(ci.NodeNameKey))

	expectedKubeBlob := map[string]interface{}{"container_name": "testContainer", "host": "testNode", "namespace_name": "testNamespace", "pod_id": "123", "pod_name": "testPod", "service_name": "testService"}
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
