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

// This needs to be reviewed for newer versions of k8s.
var failedNodeConditions = map[v1.NodeConditionType]bool{
	v1.NodeMemoryPressure:     true,
	v1.NodeDiskPressure:       true,
	v1.NodePIDPressure:        true,
	v1.NodeNetworkUnavailable: true,
}

type NodeClient interface {
	// Get the number of failed nodes for current cluster
	ClusterFailedNodeCount() int
	// Get the number of nodes for current cluster
	ClusterNodeCount() int
}

type nodeClientOption func(*nodeClient)

func nodeSyncCheckerOption(checker initialSyncChecker) nodeClientOption {
	return func(n *nodeClient) {
		n.syncChecker = checker
	}
}

type nodeClient struct {
	stopChan chan struct{}
	store    *ObjStore

	stopped     bool
	syncChecker initialSyncChecker

	mu                     sync.RWMutex
	clusterFailedNodeCount int
	clusterNodeCount       int
}

func (c *nodeClient) ClusterFailedNodeCount() int {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clusterFailedNodeCount
}

func (c *nodeClient) ClusterNodeCount() int {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clusterNodeCount
}

func (c *nodeClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

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

func newNodeClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...nodeClientOption) *nodeClient {
	c := &nodeClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}

	c.store = NewObjStore(transformFuncNode, logger)

	lw := createNodeListWatch(clientSet)
	reflector := cache.NewReflector(lw, &v1.Node{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if c.syncChecker != nil {
		// check the init sync for potential connection issue
		c.syncChecker.Check(reflector, "Node initial sync timeout")
	}

	return c
}

func (c *nodeClient) shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.stopChan)
	c.stopped = true
}

func transformFuncNode(obj any) (any, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not Node type", obj)
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
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Nodes().List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Nodes().Watch(ctx, opts)
		},
	}
}
