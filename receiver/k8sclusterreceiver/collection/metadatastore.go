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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// metadataStore keeps track of required caches exposed by informers.
// This store is used while collecting metadata about Pods to be able
// to correlate other Kubernetes objects with a Pod.
type metadataStore struct {
	services    cache.Store
	jobs        cache.Store
	replicaSets cache.Store
}

// setupStore tracks metadata of services, jobs and replicasets.
func (ms *metadataStore) setupStore(o runtime.Object, store cache.Store) {
	switch o.(type) {
	case *corev1.Service:
		ms.services = store
	case *batchv1.Job:
		ms.jobs = store
	case *appsv1.ReplicaSet:
		ms.replicaSets = store
	}
}
