// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/informer"

import (
	"errors"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type factoryKey struct {
	namespace     string
	labelSelector string
	fieldSelector string
}

type informerKey struct {
	factoryKey
	gvr schema.GroupVersionResource
}

// FactoryRegistry shares a SharedInformerFactory across observers with the same
// (namespace, labelSelector, fieldSelector) scope. The receiver builds one
// registry per leader term and calls Shutdown() to stop everything at once.
type FactoryRegistry struct {
	client       dynamic.Interface
	resyncPeriod time.Duration
	transform    cache.TransformFunc
	stopCh       chan struct{}
	stopOnce     sync.Once

	mu          sync.Mutex
	cache       map[factoryKey]dynamicinformer.DynamicSharedInformerFactory
	transformed map[informerKey]struct{}
}

// NewFactoryRegistry creates a registry backed by client. If transform is
// non-nil, it is installed on every informer returned by Informer.
func NewFactoryRegistry(client dynamic.Interface, resyncPeriod time.Duration, transform cache.TransformFunc) *FactoryRegistry {
	return &FactoryRegistry{
		client:       client,
		resyncPeriod: resyncPeriod,
		transform:    transform,
		stopCh:       make(chan struct{}),
		cache:        make(map[factoryKey]dynamicinformer.DynamicSharedInformerFactory),
		transformed:  make(map[informerKey]struct{}),
	}
}

// Get returns the factory for the given scope, creating it on first use.
// Returns an error if called after Shutdown.
func (r *FactoryRegistry) Get(namespace, labelSelector, fieldSelector string) (dynamicinformer.DynamicSharedInformerFactory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getLocked(factoryKey{namespace: namespace, labelSelector: labelSelector, fieldSelector: fieldSelector})
}

// Informer returns the shared informer for gvr in the given scope, along with
// the factory that runs it. Returns an error if called after Shutdown.
func (r *FactoryRegistry) Informer(namespace, labelSelector, fieldSelector string, gvr schema.GroupVersionResource) (dynamicinformer.DynamicSharedInformerFactory, cache.SharedIndexInformer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := factoryKey{namespace: namespace, labelSelector: labelSelector, fieldSelector: fieldSelector}
	f, err := r.getLocked(key)
	if err != nil {
		return nil, nil, err
	}
	inf := f.ForResource(gvr).Informer()
	if r.transform == nil {
		return f, inf, nil
	}
	// SetTransform fails once the informer has started, and informers are
	// shared across observers, so install it only on first use.
	ik := informerKey{factoryKey: key, gvr: gvr}
	if _, ok := r.transformed[ik]; !ok {
		if err := inf.SetTransform(r.transform); err != nil {
			return nil, nil, fmt.Errorf("failed to set transform for %s: %w", gvr, err)
		}
		r.transformed[ik] = struct{}{}
	}
	return f, inf, nil
}

func (r *FactoryRegistry) getLocked(key factoryKey) (dynamicinformer.DynamicSharedInformerFactory, error) {
	if r.cache == nil {
		return nil, errors.New("registry has been shut down")
	}
	if f, ok := r.cache[key]; ok {
		return f, nil
	}
	tweak := func(opts *metav1.ListOptions) {
		if key.labelSelector != "" {
			opts.LabelSelector = key.labelSelector
		}
		if key.fieldSelector != "" {
			opts.FieldSelector = key.fieldSelector
		}
	}
	f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.client, r.resyncPeriod, key.namespace, tweak)
	r.cache[key] = f
	return f, nil
}

// StopCh returns the channel that signals factory shutdown.
func (r *FactoryRegistry) StopCh() <-chan struct{} {
	return r.stopCh
}

// Start starts f's informers. It holds the registry lock so an informer is
// never started between its creation and SetTransform in Informer.
func (r *FactoryRegistry) Start(f dynamicinformer.DynamicSharedInformerFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f.Start(r.stopCh)
}

// Shutdown stops every factory. The registry is single-use; build a new one for the next leader term.
func (r *FactoryRegistry) Shutdown() {
	r.mu.Lock()
	factories := make([]dynamicinformer.DynamicSharedInformerFactory, 0, len(r.cache))
	for _, f := range r.cache {
		factories = append(factories, f)
	}
	r.cache = nil
	r.mu.Unlock()

	r.stopOnce.Do(func() { close(r.stopCh) })

	for _, f := range factories {
		f.Shutdown()
	}
}
