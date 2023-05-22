// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
