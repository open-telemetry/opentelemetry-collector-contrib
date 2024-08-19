// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

func TestUtils_parseDeploymentFromReplicaSet(t *testing.T) {
	assert.Equal(t, "", parseDeploymentFromReplicaSet("cloudwatch-agent"))
	assert.Equal(t, "cloudwatch-agent", parseDeploymentFromReplicaSet("cloudwatch-agent-42kcz"))
}

func TestUtils_parseCronJobFromJob(t *testing.T) {
	assert.Equal(t, "", parseCronJobFromJob("hello-123"))
	assert.Equal(t, "hello", parseCronJobFromJob("hello-1234567890"))
	assert.Equal(t, "", parseCronJobFromJob("hello-123456789a"))
}

func TestPodStore_addPodStatusMetrics(t *testing.T) {
	fields := map[string]any{}
	testPodInfo := k8sclient.PodInfo{
		Name:      "kube-proxy-csm88",
		Namespace: "kube-system",
		UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
		Labels:    map[string]string{},
		Phase:     v1.PodRunning,
	}
	addPodStatusMetrics(fields, &testPodInfo)

	expectedFieldsArray := map[string]any{
		"pod_status_pending":   0,
		"pod_status_running":   1,
		"pod_status_succeeded": 0,
		"pod_status_failed":    0,
	}
	assert.Equal(t, expectedFieldsArray, fields)
}

func TestPodStore_addPodConditionMetrics(t *testing.T) {
	fields := map[string]any{}
	testPodInfo := k8sclient.PodInfo{
		Name:      "kube-proxy-csm88",
		Namespace: "kube-system",
		UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
		Labels:    map[string]string{},
		Phase:     v1.PodRunning,
	}
	addPodConditionMetrics(fields, &testPodInfo)

	expectedFieldsArray := map[string]any{
		"pod_status_ready":     0,
		"pod_status_scheduled": 0,
		"pod_status_unknown":   0,
	}
	assert.Equal(t, expectedFieldsArray, fields)
}

func TestUtils_isHyperPodNode(t *testing.T) {
	assert.True(t, isHyperPodNode("ml.t3.medium"))
	assert.False(t, isHyperPodNode("t3.medium"))
}

func TestUtils_LabelsUtils(t *testing.T) {
	nodelabels := map[k8sclient.Label]int8{
		k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable),
	}
	status, ok := isLabelSet(int8(k8sutil.Schedulable), nodelabels, k8sclient.SageMakerNodeHealthStatus)
	assert.Equal(t, 1, status)
	assert.True(t, ok)

	status, ok = isLabelSet(int8(k8sutil.UnschedulablePendingReboot), nodelabels, k8sclient.SageMakerNodeHealthStatus)
	assert.Equal(t, 0, status)
	assert.True(t, ok)
}
