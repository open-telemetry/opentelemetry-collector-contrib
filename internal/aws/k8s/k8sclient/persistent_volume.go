// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PersistentVolumeClient interface {
	GetPersistentVolumeMetrics() *PersistentVolumeMetrics
}

type PersistentVolumeMetrics struct {
	ClusterCount int
}

type noOpPersistentVolumeClient struct{}

func (p *noOpPersistentVolumeClient) GetPersistentVolumeMetrics() *PersistentVolumeMetrics {
	return &PersistentVolumeMetrics{
		ClusterCount: 0,
	}
}

func (p *noOpPersistentVolumeClient) shutdown() {
}

type PersistentVolumeClientOption func(*PersistentVolume)

func PersistentVolumeSyncCheckerOption(checker initialSyncChecker) PersistentVolumeClientOption {
	return func(p *PersistentVolume) {
		p.syncChecker = checker
	}
}

type PersistentVolume struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu      sync.RWMutex
	metrics *PersistentVolumeMetrics
}

func (p *PersistentVolume) refresh() {
	p.mu.Lock()
	defer p.mu.Unlock()

	metrics := &PersistentVolumeMetrics{
		ClusterCount: 0,
	}

	objsList := p.store.List()
	for _, obj := range objsList {
		_, ok := obj.(*PersistentVolumeInfo)
		if !ok {
			continue
		}
		metrics.ClusterCount++
	}
	p.metrics = metrics
}

func (p *PersistentVolume) GetPersistentVolumeMetrics() *PersistentVolumeMetrics {
	if p.store.GetResetRefreshStatus() {
		p.refresh()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metrics
}

func newPersistentVolumeClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...PersistentVolumeClientOption) (*PersistentVolume, error) {
	p := &PersistentVolume{
		stopChan: make(chan struct{}),
		metrics: &PersistentVolumeMetrics{
			ClusterCount: 0,
		},
	}

	for _, option := range options {
		option(p)
	}

	ctx := context.Background()
	if _, err := clientSet.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list PersistentVolumes. err: %w", err)
	}

	// Create a store to hold PersistentVolume objects
	p.store = NewObjStore(transformFuncPersistentVolume, logger)
	// Create a ListWatch that knows how to list and watch PersistentVolumes
	lw := createPersistentVolumeListWatch(clientSet)
	// Create a Reflector that watches PersistentVolumes and updates the store
	reflector := cache.NewReflector(lw, &corev1.PersistentVolume{}, p.store, 0)
	// Start the Reflector in a goroutine
	go reflector.Run(p.stopChan)

	if p.syncChecker != nil {
		// Check the init sync for potential connection issue
		p.syncChecker.Check(reflector, "PersistentVolume initial sync timeout")
	}

	return p, nil
}

func (p *PersistentVolume) shutdown() {
	close(p.stopChan)
	p.stopped = true
}

func transformFuncPersistentVolume(obj any) (any, error) {
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not PersistentVolume type", obj)
	}

	info := &PersistentVolumeInfo{
		Name: pv.Name,
	}
	return info, nil
}

func createPersistentVolumeListWatch(client kubernetes.Interface) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().PersistentVolumes().List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().PersistentVolumes().Watch(ctx, opts)
		},
	}
}
