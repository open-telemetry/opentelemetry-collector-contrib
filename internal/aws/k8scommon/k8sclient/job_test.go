// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var jobArray = []interface{}{
	&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Name:      "job-7f8459d648",
			Namespace: "amazon-cloudwatch",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "CronJob",
					Name: "cronjobA",
					UID:  "219887d3-8d2e-11e9-9cbd-064a0c5a2714",
				},
			},
		},
	},
	&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "75ab40d2-552a-4c05-82c9-0ddcb3008657",
			Name:      "job-d6487f8459",
			Namespace: "amazon-cloudwatch",
		},
	},
}

func setUpJobClient() (*jobClient, chan struct{}) {
	stopChan := make(chan struct{})
	client := &jobClient{
		stopChan: stopChan,
		store:    NewObjStore(transformFuncJob),
		inited:   true, //make it true to avoid further initialization invocation.
	}
	return client, stopChan
}

func TestJobClient_JobToCronJob(t *testing.T) {
	client, stopChan := setUpJobClient()
	defer close(stopChan)

	client.store.Replace(jobArray, "")

	expectedMap := map[string]string{
		"job-7f8459d648": "cronjobA",
	}
	resultMap := client.JobToCronJob()
	assert.True(t, reflect.DeepEqual(resultMap, expectedMap))
}
