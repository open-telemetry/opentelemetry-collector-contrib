// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/fields"
)

func TestGetShutdown(t *testing.T) {
	tmpConfigPath := setKubeConfigPath(t)
	k8sClient := Get(
		zap.NewNop(),
		KubeConfigPath(tmpConfigPath),
		InitSyncPollInterval(10*time.Nanosecond),
		InitSyncPollTimeout(20*time.Nanosecond),
		NodeSelector(fields.OneTermEqualSelector("testField", "testVal")),
		CaptureNodeLevelInfo(true),
	)
	assert.Len(t, optionsToK8sClient, 1)
	assert.NotNil(t, k8sClient.GetClientSet())
	assert.NotNil(t, k8sClient.GetEpClient())
	assert.NotNil(t, k8sClient.GetJobClient())
	assert.NotNil(t, k8sClient.GetNodeClient())
	assert.NotNil(t, k8sClient.GetPodClient())
	assert.NotNil(t, k8sClient.GetReplicaSetClient())
	assert.True(t, k8sClient.captureNodeLevelInfo)
	assert.Equal(t, "testField=testVal", k8sClient.nodeSelector.String())
	k8sClient.Shutdown()
	assert.Nil(t, k8sClient.ep)
	assert.Nil(t, k8sClient.job)
	assert.Nil(t, k8sClient.node)
	assert.Nil(t, k8sClient.pod)
	assert.Nil(t, k8sClient.replicaSet)
	assert.Empty(t, optionsToK8sClient)
	removeTempKubeConfig()
}
