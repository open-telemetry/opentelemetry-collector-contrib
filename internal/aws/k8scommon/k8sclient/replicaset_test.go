// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var replicaSetArray = []interface{}{
	&appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "cloudwatch-agent-statsd-7f8459d648",
			Namespace: "amazon-cloudwatch",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "cloudwatch-agent-statsd",
					UID:  "219887d3-8d2e-11e9-9cbd-064a0c5a2714",
				},
			},
		},
	},
	&appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "75ab40d2-552a-4c05-82c9-0ddcb3008657",
			Name:      "cloudwatch-agent-statsd-d6487f8459",
			Namespace: "amazon-cloudwatch",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "cloudwatch-agent-statsd",
					UID:  "219887d3-8d2e-11e9-9cbd-064a0c5a2714",
				},
			},
		},
	},
}

func setUpReplicaSetClient() (*replicaSetClient, chan struct{}) {
	stopChan := make(chan struct{})
	client := &replicaSetClient{
		stopChan: stopChan,
		store:    NewObjStore(transformFuncReplicaSet),
		inited:   true, //make it true to avoid further initialization invocation.
	}
	return client, stopChan
}

func TestReplicaSetClient_ReplicaSetToDeployment(t *testing.T) {
	client, stopChan := setUpReplicaSetClient()
	defer close(stopChan)

	client.store.Replace(replicaSetArray, "")

	expectedMap := map[string]string{
		"cloudwatch-agent-statsd-7f8459d648": "cloudwatch-agent-statsd",
		"cloudwatch-agent-statsd-d6487f8459": "cloudwatch-agent-statsd",
	}
	resultMap := client.ReplicaSetToDeployment()
	assert.True(t, reflect.DeepEqual(resultMap, expectedMap))
}
