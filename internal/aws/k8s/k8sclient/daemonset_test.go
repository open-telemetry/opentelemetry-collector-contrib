// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package k8sclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

var daemonSetObjects = []runtime.Object{
	&appsv1.DaemonSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-daemonset-1",
			Namespace: "test-namespace",
			UID:       types.UID("test-daemonset-1-uid"),
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        5,
			NumberUnavailable:      3,
			DesiredNumberScheduled: 2,
			CurrentNumberScheduled: 1,
		},
	},
	&appsv1.DaemonSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-daemonset-2",
			Namespace: "test-namespace",
			UID:       types.UID("test-daemonset-2-uid"),
		},
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        10,
			NumberUnavailable:      4,
			DesiredNumberScheduled: 7,
			CurrentNumberScheduled: 7,
		},
	},
}

func TestDaemonSetClient(t *testing.T) {
	options := daemonSetSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset(daemonSetObjects...)
	client, _ := newDaemonSetClient(fakeClientSet, zap.NewNop(), options)

	daemonSets := make([]interface{}, len(daemonSetObjects))
	for i := range daemonSetObjects {
		daemonSets[i] = daemonSetObjects[i]
	}
	assert.NoError(t, client.store.Replace(daemonSets, ""))

	expected := []*DaemonSetInfo{
		{
			Name:      "test-daemonset-1",
			Namespace: "test-namespace",
			Status: &DaemonSetStatus{
				NumberAvailable:        5,
				NumberUnavailable:      3,
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 1,
			},
		},
		{
			Name:      "test-daemonset-2",
			Namespace: "test-namespace",
			Status: &DaemonSetStatus{
				NumberAvailable:        10,
				NumberUnavailable:      4,
				DesiredNumberScheduled: 7,
				CurrentNumberScheduled: 7,
			},
		},
	}
	actual := client.DaemonSetInfos()
	assert.EqualValues(t, expected, actual)
	client.shutdown()
	assert.True(t, client.stopped)
}
