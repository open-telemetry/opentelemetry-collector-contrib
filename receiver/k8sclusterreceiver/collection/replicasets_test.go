// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newReplicaSet(id string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: v1.ObjectMeta{
			Name:        "test-replicaset-" + id,
			Namespace:   "test-namespace",
			UID:         types.UID("test-replicaset-" + id + "-uid"),
			ClusterName: "test-cluster",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Spec:   appsv1.ReplicaSetSpec{},
		Status: appsv1.ReplicaSetStatus{},
	}
}
