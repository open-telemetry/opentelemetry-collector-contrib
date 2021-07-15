// Copyright 2020 OpenTelemetry Authors
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

package collection

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func mockMetadataStore(to testCaseOptions) *metadataStore {
	ms := &metadataStore{}

	if to.wantNilCache {
		return ms
	}

	store := &testutils.MockStore{
		Cache:   map[string]interface{}{},
		WantErr: to.wantErrFromCache,
	}

	switch to.kind {
	case "Job":
		ms.jobs = store
		if !to.emptyCache {
			if to.withParentOR {
				store.Cache["test-namespace/test-job-0"] = withOwnerReferences(
					[]v1.OwnerReference{
						{
							Kind: "CronJob",
							Name: "test-cronjob-0",
							UID:  "test-cronjob-0-uid",
						},
					}, newJob("0"),
				)
			} else {
				store.Cache["test-namespace/test-job-0"] = newJob("0")
			}
		}
		return ms
	case "ReplicaSet":
		ms.replicaSets = store
		if !to.emptyCache {
			if to.withParentOR {
				store.Cache["test-namespace/test-replicaset-0"] = withOwnerReferences(
					[]v1.OwnerReference{
						{
							Kind: "Deployment",
							Name: "test-deployment-0",
							UID:  "test-deployment-0-uid",
						},
					}, newReplicaSet("0"),
				)
			} else {
				store.Cache["test-namespace/test-replicaset-0"] = newReplicaSet("0")
			}
		}
		return ms
	}

	return ms
}

func withOwnerReferences(or []v1.OwnerReference, obj interface{}) interface{} {
	switch o := obj.(type) {
	case *corev1.Pod:
		o.OwnerReferences = or
		return o
	case *batchv1.Job:
		o.OwnerReferences = or
		return o
	case *appsv1.ReplicaSet:
		o.OwnerReferences = or
		return o
	}
	return obj
}

func podWithAdditionalLabels(labels map[string]string, pod *corev1.Pod) interface{} {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string, len(labels))
	}

	for k, v := range labels {
		pod.Labels[k] = v
	}

	return pod
}

func podWithOwnerReference(kind string) interface{} {
	kindLower := strings.ToLower(kind)
	return withOwnerReferences(
		[]v1.OwnerReference{
			{
				Kind: kind,
				Name: fmt.Sprintf("test-%s-0", kindLower),
				UID:  types.UID(fmt.Sprintf("test-%s-0-uid", kindLower)),
			},
		}, newPodWithContainer("0", &corev1.PodSpec{}, &corev1.PodStatus{}),
	)
}
