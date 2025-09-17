// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const ClusterWideInformerKey = "<cluster-wide-informer-key>"

// Store keeps track of required caches exposed by informers.
// This store is used while collecting metadata about Pods to be able
// to correlate other Kubernetes objects with a Pod.
type Store struct {
	stores map[schema.GroupVersionKind]map[string]cache.Store
}

// NewStore creates a new Store.
func NewStore() *Store {
	return &Store{
		stores: make(map[schema.GroupVersionKind]map[string]cache.Store),
	}
}

// Get returns a cache.Store for a given GroupVersionKind.
func (ms *Store) Get(gvk schema.GroupVersionKind) map[string]cache.Store {
	return ms.stores[gvk]
}

// Setup tracks metadata of services, jobs and replicasets.
func (ms *Store) Setup(gvk schema.GroupVersionKind, namespace string, store cache.Store) {
	if _, ok := ms.stores[gvk]; !ok {
		ms.stores[gvk] = make(map[string]cache.Store)
	}
	ms.stores[gvk][namespace] = store
}

// ForEach iterates over all objects in a given cache.Store.
func (ms *Store) ForEach(gvk schema.GroupVersionKind, f func(o any)) {
	for _, store := range ms.stores[gvk] {
		for _, obj := range store.List() {
			f(obj)
		}
	}
}
