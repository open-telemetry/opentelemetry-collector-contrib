// Copyright The OpenTelemetry Authors
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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
)

// Store keeps track of required caches exposed by informers.
// This store is used while collecting metadata about Pods to be able
// to correlate other Kubernetes objects with a Pod.
type Store struct {
	Services    cache.Store
	Jobs        cache.Store
	ReplicaSets cache.Store
}

// Setup tracks metadata of services, jobs and replicasets.
func (ms *Store) Setup(kind schema.GroupVersionKind, store cache.Store) {
	switch kind {
	case gvk.Service:
		ms.Services = store
	case gvk.Job:
		ms.Jobs = store
	case gvk.ReplicaSet:
		ms.ReplicaSets = store
	}
}
