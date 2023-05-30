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

package k8sapiserver

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

func TestNewLeaderElectionOpts(t *testing.T) {
	t.Setenv("HOST_NAME", "hostname")
	t.Setenv("K8S_NAMESPACE", "namespace")

	le, err := NewLeaderElection(zap.NewNop(), WithLeaderLockName("test"), WithLeaderLockUsingConfigMapOnly(true))
	assert.NotNil(t, le)
	assert.NoError(t, err)
	assert.Equal(t, "test", le.leaderLockName)
	assert.True(t, le.leaderLockUsingConfigMapOnly)

}
func TestLeaderElectionInitErrors(t *testing.T) {
	le, err := NewLeaderElection(zap.NewNop())
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "environment variable HOST_NAME is not set"))
	assert.Nil(t, le)

	t.Setenv("HOST_NAME", "hostname")

	le, err = NewLeaderElection(zap.NewNop())
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "environment variable K8S_NAMESPACE is not set"))
	assert.Nil(t, le)
}

func TestLeaderElectionEndToEnd(t *testing.T) {
	hostName, err := os.Hostname()
	assert.NoError(t, err)
	k8sclientOption := func(le *LeaderElection) {
		le.k8sClient = &mockK8sClient{}
	}
	leadingOption := func(le *LeaderElection) {
		le.leading = true
	}
	broadcasterOption := func(le *LeaderElection) {
		le.broadcaster = &mockEventBroadcaster{}
	}
	isLeadingCOption := func(le *LeaderElection) {
		le.isLeadingC = make(chan bool)
	}

	t.Setenv("HOST_NAME", hostName)
	t.Setenv("K8S_NAMESPACE", "namespace")
	leaderElection, err := NewLeaderElection(zap.NewNop(), k8sclientOption,
		leadingOption, broadcasterOption, isLeadingCOption)

	assert.NotNil(t, leaderElection)
	assert.NoError(t, err)

	mockClient.On("NamespaceToRunningPodNum").Return(map[string]int{"default": 2})
	mockClient.On("ClusterFailedNodeCount").Return(1)
	mockClient.On("ClusterNodeCount").Return(1)
	mockClient.On("ServiceToPodNum").Return(
		map[k8sclient.Service]int{
			NewService("service1", "kube-system"): 1,
			NewService("service2", "kube-system"): 1,
		},
	)

	<-leaderElection.isLeadingC
	assert.True(t, leaderElection.leading)

	// shut down
	leaderElection.cancel()

	assert.Eventually(t, func() bool {
		return !leaderElection.leading
	}, 2*time.Second, 5*time.Millisecond)

}
