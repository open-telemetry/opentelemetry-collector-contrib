// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetShutdown(t *testing.T) {
	tmpConfigPath := setKubeConfigPath(t)
	k8sClient := Get(
		zap.NewNop(),
		KubeConfigPath(tmpConfigPath),
		InitSyncPollInterval(10*time.Nanosecond),
		InitSyncPollTimeout(20*time.Nanosecond),
	)
	assert.Equal(t, 1, len(optionsToK8sClient))
	assert.NotNil(t, k8sClient.GetClientSet())
	assert.NotNil(t, k8sClient.GetEpClient())
	assert.NotNil(t, k8sClient.GetJobClient())
	assert.NotNil(t, k8sClient.GetNodeClient())
	assert.NotNil(t, k8sClient.GetPodClient())
	assert.NotNil(t, k8sClient.GetReplicaSetClient())
	k8sClient.Shutdown()
	assert.Nil(t, k8sClient.ep)
	assert.Nil(t, k8sClient.job)
	assert.Nil(t, k8sClient.node)
	assert.Nil(t, k8sClient.pod)
	assert.Nil(t, k8sClient.replicaSet)
	assert.Equal(t, 0, len(optionsToK8sClient))
	removeTempKubeConfig()
}
