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
	k8sClient.Shutdown()
	assert.Nil(t, k8sClient.Ep)
	assert.Nil(t, k8sClient.Job)
	assert.Nil(t, k8sClient.Node)
	assert.Nil(t, k8sClient.Pod)
	assert.Nil(t, k8sClient.ReplicaSet)
	assert.Equal(t, 0, len(optionsToK8sClient))
	removeTempKubeConfig()
}
