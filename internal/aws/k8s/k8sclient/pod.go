// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodClient interface {
	// Get the mapping between the namespace and the number of belonging pods
	NamespaceToRunningPodNum() map[string]int
}

type podClientOption func(*podClient)

func podSyncCheckerOption(checker initialSyncChecker) podClientOption {
	return func(p *podClient) {
		p.syncChecker = checker
	}
}

type podClient struct {
	stopChan chan struct{}
	store    *ObjStore

	stopped     bool
	syncChecker initialSyncChecker

	mu                          sync.RWMutex
	namespaceToRunningPodNumMap map[string]int
}

func (c *podClient) NamespaceToRunningPodNum() map[string]int {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.namespaceToRunningPodNumMap
}

func (c *podClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	objsList := c.store.List()
	namespaceToRunningPodNumMapNew := make(map[string]int)
	for _, obj := range objsList {
		pod := obj.(*podInfo)
		if pod.phase == v1.PodRunning {
			if podNum, ok := namespaceToRunningPodNumMapNew[pod.namespace]; !ok {
				namespaceToRunningPodNumMapNew[pod.namespace] = 1
			} else {
				namespaceToRunningPodNumMapNew[pod.namespace] = podNum + 1
			}
		}
	}
	c.namespaceToRunningPodNumMap = namespaceToRunningPodNumMapNew
}

func newPodClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...podClientOption) *podClient {
	c := &podClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}

	c.store = NewObjStore(transformFuncPod, logger)

	lw := createPodListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &v1.Pod{}, c.store, 0)

	go reflector.Run(c.stopChan)

	if c.syncChecker != nil {
		// check the init sync for potential connection issue
		c.syncChecker.Check(reflector, "Pod initial sync timeout")
	}

	return c
}

func (c *podClient) shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.stopChan)
	c.stopped = true
}

func transformFuncPod(obj interface{}) (interface{}, error) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not Pod type", obj)
	}
	info := new(podInfo)
	info.namespace = pod.Namespace
	info.phase = pod.Status.Phase
	return info, nil
}

func createPodListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Pods(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Pods(ns).Watch(ctx, opts)
		},
	}
}
