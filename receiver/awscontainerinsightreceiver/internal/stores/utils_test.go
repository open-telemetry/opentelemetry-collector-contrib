// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

type mockCIMetric struct {
	tags   map[string]string
	fields map[string]any
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

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	assert.Empty(t, parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	assert.Empty(t, parseCronJobFromJob("hello-123"))
	assert.Equal(t, "hello", parseCronJobFromJob("hello-1234567890"))
	assert.Empty(t, parseCronJobFromJob("hello-123456789a"))
}

func TestUtils_addKubernetesInfo(t *testing.T) {
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
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

	kubernetesBlob := map[string]any{}
	AddKubernetesInfo(metric, kubernetesBlob)
	assert.Empty(t, metric.GetTag(ci.ContainerNamekey))
	assert.Empty(t, metric.GetTag(ci.K8sPodNameKey))
	assert.Empty(t, metric.GetTag(ci.PodIDKey))
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
