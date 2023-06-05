// Copyright The OpenTelemetry Authors
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
	assert.Equal(t, 1, len(optionsToK8sClient))
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
	assert.Equal(t, 0, len(optionsToK8sClient))
	removeTempKubeConfig()
}
