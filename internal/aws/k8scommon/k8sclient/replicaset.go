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

	// "github.com/aws/amazon-cloudwatch-agent/internal/containerinsightscommon"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	Deployment = "Deployment"
)

type ReplicaSetClient interface {
	ReplicaSetToDeployment() map[string]string

	Init()
	Shutdown()
}

type replicaSetClient struct {
	sync.RWMutex

	stopChan chan struct{}
	store    *ObjStore

	inited bool

	cachedReplicaSetMap       map[string]time.Time
	replicaSetToDeploymentMap map[string]string
}

func (c *replicaSetClient) ReplicaSetToDeployment() map[string]string {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.replicaSetToDeploymentMap
}

func (c *replicaSetClient) refresh() {
	c.Lock()
	defer c.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]string)
	for _, obj := range objsList {
		replicaSet := obj.(*replicaSetInfo)
	ownerLoop:
		for _, owner := range replicaSet.owners {
			if owner.kind == Deployment && owner.name != "" {
				tmpMap[replicaSet.name] = owner.name
				break ownerLoop
			}
		}
	}

	if c.replicaSetToDeploymentMap == nil {
		c.replicaSetToDeploymentMap = make(map[string]string)
	}

	if c.cachedReplicaSetMap == nil {
		c.cachedReplicaSetMap = make(map[string]time.Time)
	}

	lastRefreshTime := time.Now()

	for k, v := range c.cachedReplicaSetMap {
		if lastRefreshTime.Sub(v) > cacheTTL {
			delete(c.replicaSetToDeploymentMap, k)
			delete(c.cachedReplicaSetMap, k)
		}
	}

	for k, v := range tmpMap {
		c.replicaSetToDeploymentMap[k] = v
		c.cachedReplicaSetMap[k] = lastRefreshTime
	}
}

func (c *replicaSetClient) Init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	ctx, _ := context.WithCancel(context.Background())
	if _, err := Get().ClientSet.AppsV1().ReplicaSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		panic(fmt.Sprintf("Cannot list ReplicaSet. err: %v", err))
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncReplicaSet)

	lw := createReplicaSetListWatch(Get().ClientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &appsv1.ReplicaSet{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if err := wait.Poll(50*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		log.Printf("W! ReplicaSet initial sync timeout: %v", err)
	}

	c.inited = true
}

func (c *replicaSetClient) Shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}

	close(c.stopChan)

	c.inited = false
}

func transformFuncReplicaSet(obj interface{}) (interface{}, error) {
	replicaSet, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		return nil, errors.New(fmt.Sprintf("input obj %v is not ReplicaSet type", obj))
	}
	info := new(replicaSetInfo)
	info.name = replicaSet.Name
	info.owners = []*replicaSetOwner{}
	for _, owner := range replicaSet.OwnerReferences {
		info.owners = append(info.owners, &replicaSetOwner{kind: owner.Kind, name: owner.Name})
	}
	return info, nil
}

func createReplicaSetListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx, _ := context.WithCancel(context.Background())
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.AppsV1().ReplicaSets(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.AppsV1().ReplicaSets(ns).Watch(ctx, opts)
		},
	}
}
