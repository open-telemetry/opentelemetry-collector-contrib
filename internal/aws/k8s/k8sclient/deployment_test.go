// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package k8sclient

import (
	"sort"
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
			Replicas:            10,
			ReadyReplicas:       5,
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
			ReadyReplicas:       10,
			AvailableReplicas:   10,
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
				Replicas:            10,
				ReadyReplicas:       5,
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
				ReadyReplicas:       10,
				AvailableReplicas:   10,
				UnavailableReplicas: 2,
			},
		},
	}
	actual := client.DeploymentInfos()
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].Name < actual[j].Name
	})
	assert.Equal(t, expected, actual)
	client.shutdown()
	assert.True(t, client.stopped)
}
