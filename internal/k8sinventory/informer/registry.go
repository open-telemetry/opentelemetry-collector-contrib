// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package informer // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory/informer"

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

type factoryKey struct {
	namespace     string
	labelSelector string
	fieldSelector string
}

// FactoryRegistry shares a SharedInformerFactory across observers with the same
// (namespace, labelSelector, fieldSelector) scope. The receiver builds one
// registry per leader term and calls Shutdown() to stop everything at once.
type FactoryRegistry struct {
	client dynamic.Interface
	stopCh chan struct{}

	mu    sync.Mutex
	cache map[factoryKey]dynamicinformer.DynamicSharedInformerFactory
}

// NewFactoryRegistry creates a registry backed by client.
func NewFactoryRegistry(client dynamic.Interface) *FactoryRegistry {
	return &FactoryRegistry{
		client: client,
		stopCh: make(chan struct{}),
		cache:  make(map[factoryKey]dynamicinformer.DynamicSharedInformerFactory),
	}
}

// Get returns the factory for the given scope, creating it on first use.
func (r *FactoryRegistry) Get(namespace, labelSelector, fieldSelector string) dynamicinformer.DynamicSharedInformerFactory {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := factoryKey{namespace: namespace, labelSelector: labelSelector, fieldSelector: fieldSelector}
	if f, ok := r.cache[key]; ok {
		return f
	}
	tweak := func(opts *metav1.ListOptions) {
		if key.labelSelector != "" {
			opts.LabelSelector = key.labelSelector
		}
		if key.fieldSelector != "" {
			opts.FieldSelector = key.fieldSelector
		}
	}
	// Resync period 0: pull-mode tickers handle their own cadence; we don't need periodic re-emits.
	f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.client, 0, key.namespace, tweak)
	r.cache[key] = f
	return f
}

// StopCh returns the channel that signals factory shutdown. Observers pass it to factory.Start.
func (r *FactoryRegistry) StopCh() <-chan struct{} {
	return r.stopCh
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

	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}

	for _, f := range factories {
		f.Shutdown()
	}
}
