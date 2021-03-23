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

// This needs to be reviewed for newer versions of k8s.
var failedNodeConditions = map[v1.NodeConditionType]bool{
	//v1.NodeOutOfDisk:          true,
	v1.NodeMemoryPressure:     true,
	v1.NodeDiskPressure:       true,
	v1.NodePIDPressure:        true,
	v1.NodeNetworkUnavailable: true,
}

type NodeClient interface {
	ClusterFailedNodeCount() int
	ClusterNodeCount() int

	Init()
	Shutdown()
}

type nodeClient struct {
	sync.RWMutex

	stopChan chan struct{}
	store    *ObjStore

	inited bool

	clusterFailedNodeCount int
	clusterNodeCount       int
}

func (c *nodeClient) ClusterFailedNodeCount() int {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.clusterFailedNodeCount
}

func (c *nodeClient) ClusterNodeCount() int {
	if !c.inited {
		c.Init()
	}
	if c.store.Refreshed() {
		c.refresh()
	}
	c.RLock()
	defer c.RUnlock()
	return c.clusterNodeCount
}

func (c *nodeClient) refresh() {
	c.Lock()
	defer c.Unlock()

	objsList := c.store.List()

	clusterFailedNodeCountNew := 0
	clusterNodeCountNew := 0
	for _, obj := range objsList {
		node := obj.(*nodeInfo)

		clusterNodeCountNew++

		failed := false

	Loop:
		for _, condition := range node.conditions {
			if _, ok := failedNodeConditions[condition.Type]; ok {
				// match the failedNodeConditions type we care about
				if condition.Status != v1.ConditionFalse {
					// if this is not false, i.e. true or unknown
					failed = true
					break Loop
				}
			}
		}

		if failed {
			clusterFailedNodeCountNew++
		}
	}

	c.clusterFailedNodeCount = clusterFailedNodeCountNew
	c.clusterNodeCount = clusterNodeCountNew
}

func (c *nodeClient) Init() {
	c.Lock()
	defer c.Unlock()
	if c.inited {
		return
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncNode)

	lw := createNodeListWatch(Get().ClientSet)
	reflector := cache.NewReflector(lw, &v1.Node{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if err := wait.Poll(50*time.Millisecond, 2*time.Second, func() (done bool, err error) {
		return reflector.LastSyncResourceVersion() != "", nil
	}); err != nil {
		log.Printf("W! Node initial sync timeout: %v", err)
	}

	c.inited = true
}

func (c *nodeClient) Shutdown() {
	c.Lock()
	defer c.Unlock()
	if !c.inited {
		return
	}

	close(c.stopChan)

	c.inited = false
}

func transformFuncNode(obj interface{}) (interface{}, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return nil, errors.New(fmt.Sprintf("input obj %v is not Node type", obj))
	}
	info := new(nodeInfo)
	info.conditions = []*nodeCondition{}
	for _, condition := range node.Status.Conditions {
		info.conditions = append(info.conditions, &nodeCondition{
			Type:   condition.Type,
			Status: condition.Status,
		})
	}
	return info, nil
}

func createNodeListWatch(client kubernetes.Interface) cache.ListerWatcher {
	ctx, _ := context.WithCancel(context.Background())
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Nodes().List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Nodes().Watch(ctx, opts)
		},
	}
}
