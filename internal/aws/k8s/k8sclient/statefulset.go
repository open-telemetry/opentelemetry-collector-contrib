// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type StatefulSetClient interface {
	// StatefulSetInfos contains the information about each statefulSet in the cluster
	StatefulSetInfos() []*StatefulSetInfo
}

type noOpStatefulSetClient struct{}

func (nd *noOpStatefulSetClient) StatefulSetInfos() []*StatefulSetInfo {
	return []*StatefulSetInfo{}
}

func (nd *noOpStatefulSetClient) shutdown() {
}

type statefulSetClientOption func(*statefulSetClient)

func statefulSetSyncCheckerOption(checker initialSyncChecker) statefulSetClientOption {
	return func(d *statefulSetClient) {
		d.syncChecker = checker
	}
}

type statefulSetClient struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu               sync.RWMutex
	statefulSetInfos []*StatefulSetInfo
}

func (d *statefulSetClient) refresh() {
	d.mu.Lock()
	defer d.mu.Unlock()

	var statefulSetInfos []*StatefulSetInfo
	objsList := d.store.List()
	for _, obj := range objsList {
		statefulSet, ok := obj.(*StatefulSetInfo)
		if !ok {
			continue
		}
		statefulSetInfos = append(statefulSetInfos, statefulSet)
	}

	d.statefulSetInfos = statefulSetInfos
}

func (d *statefulSetClient) StatefulSetInfos() []*StatefulSetInfo {
	if d.store.GetResetRefreshStatus() {
		d.refresh()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.statefulSetInfos
}

func newStatefulSetClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...statefulSetClientOption) (*statefulSetClient, error) {
	d := &statefulSetClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(d)
	}

	ctx := context.Background()
	if _, err := clientSet.AppsV1().StatefulSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list StatefulSets. err: %w", err)
	}

	d.store = NewObjStore(transformFuncStatefulSet, logger)
	lw := createStatefulSetListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &appsv1.StatefulSet{}, d.store, 0)

	go reflector.Run(d.stopChan)

	if d.syncChecker != nil {
		// check the init sync for potential connection issue
		d.syncChecker.Check(reflector, "StatefulSet initial sync timeout")
	}

	return d, nil
}

func (d *statefulSetClient) shutdown() {
	close(d.stopChan)
	d.stopped = true
}

func transformFuncStatefulSet(obj any) (any, error) {
	statefulSet, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not StatefulSet type", obj)
	}
	info := new(StatefulSetInfo)
	info.Name = statefulSet.Name
	info.Namespace = statefulSet.Namespace
	info.Spec = &StatefulSetSpec{
		Replicas: uint32(*statefulSet.Spec.Replicas),
	}
	info.Status = &StatefulSetStatus{
		Replicas:          uint32(statefulSet.Status.Replicas),
		AvailableReplicas: uint32(statefulSet.Status.AvailableReplicas),
		ReadyReplicas:     uint32(statefulSet.Status.ReadyReplicas),
	}
	return info, nil
}

func createStatefulSetListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.AppsV1().StatefulSets(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.AppsV1().StatefulSets(ns).Watch(ctx, opts)
		},
	}
}
