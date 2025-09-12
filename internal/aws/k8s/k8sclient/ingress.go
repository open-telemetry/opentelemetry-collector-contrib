// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressClient interface {
	GetIngressMetrics() *IngressMetrics
}

type IngressMetrics struct {
	NamespaceCount map[string]int
}

type noOpIngressClient struct{}

func (n *noOpIngressClient) GetIngressMetrics() *IngressMetrics {
	return &IngressMetrics{
		NamespaceCount: make(map[string]int),
	}
}

func (n *noOpIngressClient) shutdown() {
}

type ingressClientOption func(*ingressClient)

func ingressSyncCheckerOption(checker initialSyncChecker) ingressClientOption {
	return func(c *ingressClient) {
		c.syncChecker = checker
	}
}

type ingressClient struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu      sync.RWMutex
	metrics *IngressMetrics
}

func (d *ingressClient) refresh() {
	d.mu.Lock()
	defer d.mu.Unlock()

	metrics := &IngressMetrics{
		NamespaceCount: make(map[string]int),
	}
	objsList := d.store.List()
	for _, obj := range objsList {
		igInfo, ok := obj.(*IngressInfo)
		if !ok {
			continue
		}
		metrics.NamespaceCount[igInfo.Namespace]++
	}

	d.metrics = metrics
}

func (d *ingressClient) GetIngressMetrics() *IngressMetrics {
	if d.store.GetResetRefreshStatus() {
		d.refresh()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.metrics
}

func (d *ingressClient) shutdown() {
	close(d.stopChan)
	d.stopped = true
}

func newIngressClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...ingressClientOption) (*ingressClient, error) {
	d := &ingressClient{
		stopChan: make(chan struct{}),
		metrics: &IngressMetrics{
			NamespaceCount: make(map[string]int),
		},
	}

	for _, option := range options {
		option(d)
	}

	ctx := context.Background()
	if _, err := clientSet.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ingress. err: %w", err)
	}

	d.store = NewObjStore(transformFuncIngress, logger)
	lw := createIngressListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &networkingv1.Ingress{}, d.store, 0)

	go reflector.Run(d.stopChan)

	if d.syncChecker != nil {
		// check the init sync for potential connection issue
		d.syncChecker.Check(reflector, "Ingress initial sync timeout")
	}

	return d, nil
}

func transformFuncIngress(obj any) (any, error) {
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil, fmt.Errorf("object is not an ingress: %+v", obj)
	}
	info := &IngressInfo{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
	}
	return info, nil
}

func createIngressListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.NetworkingV1().Ingresses(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.NetworkingV1().Ingresses(ns).Watch(ctx, opts)
		},
	}
}
