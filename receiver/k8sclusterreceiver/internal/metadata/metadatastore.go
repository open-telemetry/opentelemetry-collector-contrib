// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// Store keeps track of required caches exposed by informers.
// This store is used while collecting metadata about Pods to be able
// to correlate other Kubernetes objects with a Pod.
type Store struct {
	stores map[schema.GroupVersionKind]cache.Store
}

// NewStore creates a new Store.
func NewStore() *Store {
	return &Store{
		stores: make(map[schema.GroupVersionKind]cache.Store),
	}
}

// Get returns a cache.Store for a given GroupVersionKind.
func (ms *Store) Get(gvk schema.GroupVersionKind) cache.Store {
	return ms.stores[gvk]
}

// Setup tracks metadata of services, jobs and replicasets.
func (ms *Store) Setup(gvk schema.GroupVersionKind, store cache.Store) {
	ms.stores[gvk] = store
}

// ForEach iterates over all objects in a given cache.Store.
func (ms *Store) ForEach(gvk schema.GroupVersionKind, f func(o any)) {
	store := ms.Get(gvk)
	if store == nil {
		// This is normal, not all caches are set up, e.g. ClusterResourceQuota is only available in OpenShift.
		return
	}
	for _, obj := range store.List() {
		f(obj)
	}
}
