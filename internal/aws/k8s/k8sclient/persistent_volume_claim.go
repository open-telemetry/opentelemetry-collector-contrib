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

// PersistentVolumeClaimMetrics holds all the metrics for PVCs
type PersistentVolumeClaimMetrics struct {
	PersistentVolumeClaimPhases map[string]corev1.PersistentVolumeClaimPhase // Individual PVC metrics
}

type PersistentVolumeClaimClient interface {
	GetPersistentVolumeClaimMetrics() *PersistentVolumeClaimMetrics
}

type noOpPersistentVolumeClaimClient struct{}

func (p *noOpPersistentVolumeClaimClient) GetPersistentVolumeClaimMetrics() *PersistentVolumeClaimMetrics {
	return &PersistentVolumeClaimMetrics{
		PersistentVolumeClaimPhases: make(map[string]corev1.PersistentVolumeClaimPhase),
	}
}

func (p *noOpPersistentVolumeClaimClient) shutdown() {
}

type PersistentVolumeClaimClientOption func(*PersistentVolumeClaim)

func PersistentVolumeClaimSyncCheckerOption(checker initialSyncChecker) PersistentVolumeClaimClientOption {
	return func(p *PersistentVolumeClaim) {
		p.syncChecker = checker
	}
}

type PersistentVolumeClaim struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu      sync.RWMutex
	metrics *PersistentVolumeClaimMetrics
}

func (p *PersistentVolumeClaim) refresh() {
	p.mu.Lock()
	defer p.mu.Unlock()

	metrics := &PersistentVolumeClaimMetrics{
		PersistentVolumeClaimPhases: make(map[string]corev1.PersistentVolumeClaimPhase),
	}

	objsList := p.store.List()
	for _, obj := range objsList {
		pvcInfo, ok := obj.(*PersistentVolumeClaimInfo)
		if !ok {
			continue
		}

		pvcKey := fmt.Sprintf("%s/%s", pvcInfo.Namespace, pvcInfo.Name)
		phase := pvcInfo.Status.Phase
		metrics.PersistentVolumeClaimPhases[pvcKey] = phase
	}
	p.metrics = metrics
}

func (p *PersistentVolumeClaim) GetPersistentVolumeClaimMetrics() *PersistentVolumeClaimMetrics {
	if p.store.GetResetRefreshStatus() {
		p.refresh()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.metrics
}

func newPersistentVolumeClaimClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...PersistentVolumeClaimClientOption) (*PersistentVolumeClaim, error) {
	p := &PersistentVolumeClaim{
		stopChan: make(chan struct{}),
		metrics: &PersistentVolumeClaimMetrics{
			PersistentVolumeClaimPhases: make(map[string]corev1.PersistentVolumeClaimPhase),
		},
	}

	for _, option := range options {
		option(p)
	}

	ctx := context.Background()
	if _, err := clientSet.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list PVCs. err: %w", err)
	}

	// Create a store to hold PVC objects
	p.store = NewObjStore(transformFuncPersistentVolumeClaim, logger)
	// Create a ListWatch that knows how to list and watch PVCs
	lw := createPersistentVolumeClaimListWatch(clientSet, metav1.NamespaceAll)
	// Create a Reflector that watches PVCs and updates the store
	reflector := cache.NewReflector(lw, &corev1.PersistentVolumeClaim{}, p.store, 0)
	// Start the Reflector in a goroutine
	go reflector.Run(p.stopChan)

	if p.syncChecker != nil {
		// Check the init sync for potential connection issue
		p.syncChecker.Check(reflector, "PersistentVolumeClaim initial sync timeout")
	}

	return p, nil
}

func (p *PersistentVolumeClaim) shutdown() {
	close(p.stopChan)
	p.stopped = true
}

func transformFuncPersistentVolumeClaim(obj any) (any, error) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not PersistentVolumeClaim type", obj)
	}

	info := &PersistentVolumeClaimInfo{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
		Status: &PersistentVolumeClaimStatus{
			Phase: pvc.Status.Phase,
		},
	}
	return info, nil
}

func createPersistentVolumeClaimListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().PersistentVolumeClaims(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().PersistentVolumeClaims(ns).Watch(ctx, opts)
		},
	}
}
