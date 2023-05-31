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

var desired = int32(20)

var deploymentObjects = []runtime.Object{
	&appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-deployment-1",
			Namespace: "test-namespace",
			UID:       types.UID("test-deployment-1-uid"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            5,
			AvailableReplicas:   5,
			UnavailableReplicas: 1,
		},
	},
	&appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-deployment-2",
			Namespace: "test-namespace",
			UID:       types.UID("test-deployment-2-uid"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desired,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            15,
			AvailableReplicas:   15,
			UnavailableReplicas: 2,
		},
	},
}

func TestDeploymentClient(t *testing.T) {
	options := deploymentSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewSimpleClientset(deploymentObjects...)
	client, _ := newDeploymentClient(fakeClientSet, zap.NewNop(), options)

	deployments := make([]interface{}, len(deploymentObjects))
	for i := range deploymentObjects {
		deployments[i] = deploymentObjects[i]
	}
	assert.NoError(t, client.store.Replace(deployments, ""))

	expected := []*DeploymentInfo{
		{
			Name:      "test-deployment-1",
			Namespace: "test-namespace",
			Spec: &DeploymentSpec{
				Replicas: 20,
			},
			Status: &DeploymentStatus{
				Replicas:            5,
				AvailableReplicas:   5,
				UnavailableReplicas: 1,
			},
		},
		{
			Name:      "test-deployment-2",
			Namespace: "test-namespace",
			Spec: &DeploymentSpec{
				Replicas: 20,
			},
			Status: &DeploymentStatus{
				Replicas:            15,
				AvailableReplicas:   15,
				UnavailableReplicas: 2,
			},
		},
	}
	actual := client.DeploymentInfos()
	assert.EqualValues(t, expected, actual)
	client.shutdown()
	assert.True(t, client.stopped)
}
