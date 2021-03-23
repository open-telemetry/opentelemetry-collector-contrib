// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodClient interface {
	NamespaceToRunningPodNum() map[string]int

	Init()
	Shutdown()
}

type podClient struct {
	sync.RWMutex

	stopChan chan struct{}
	store    *ObjStore

	inited bool

	namespaceToRunningPodNumMap map[string]int
}

func (c *podClient) NamespaceToRunningPodNum() map[string]int {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.namespaceToRunningPodNumMap
}

func (c *podClient) refresh() {
	c.Lock()
	defer c.Unlock()

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

func (c *podClient) Init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncPod)

	lw := createPodListWatch(Get().ClientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &v1.Pod{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if err := wait.Poll(50*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		log.Printf("W! Pod initial sync timeout: %v", err)
	}

	c.inited = true
}

func (c *podClient) Shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}

	close(c.stopChan)

	c.inited = false
}

func transformFuncPod(obj interface{}) (interface{}, error) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil, errors.New(fmt.Sprintf("input obj %v is not Pod type", obj))
	}
	info := new(podInfo)
	info.namespace = pod.Namespace
	info.phase = pod.Status.Phase
	return info, nil
}

func createPodListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx, _ := context.WithCancel(context.Background())
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Pods(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Pods(ns).Watch(ctx, opts)
		},
	}
}
