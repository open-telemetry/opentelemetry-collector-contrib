// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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
	NodeToCapacityMap() map[string]v1.ResourceList
	NodeToAllocatableMap() map[string]v1.ResourceList
	NodeToConditionsMap() map[string]map[v1.NodeConditionType]v1.ConditionStatus
}

type nodeClientOption func(*nodeClient)

func nodeSyncCheckerOption(checker initialSyncChecker) nodeClientOption {
	return func(n *nodeClient) {
		n.syncChecker = checker
	}
}

func nodeSelectorOption(nodeSelector fields.Selector) nodeClientOption {
	return func(n *nodeClient) {
		n.nodeSelector = nodeSelector
	}
}

func captureNodeLevelInfoOption(captureNodeLevelInfo bool) nodeClientOption {
	return func(n *nodeClient) {
		n.captureNodeLevelInfo = captureNodeLevelInfo
	}
}

type nodeClient struct {
	stopChan chan struct{}
	store    *ObjStore
	logger   *zap.Logger

	stopped     bool
	syncChecker initialSyncChecker

	nodeSelector fields.Selector

	// The node client can be used in several places, including code paths that execute on both leader and non-leader nodes.
	// But for logic on the leader node (for ex in k8sapiserver.go), there is no need to obtain node level info since only cluster
	// level info is needed there. Hence, this optimization allows us to save on memory by not capturing node level info when not needed.
	captureNodeLevelInfo bool

	mu                     sync.RWMutex
	clusterFailedNodeCount int
	clusterNodeCount       int
	nodeToCapacityMap      map[string]v1.ResourceList
	nodeToAllocatableMap   map[string]v1.ResourceList
	nodeToConditionsMap    map[string]map[v1.NodeConditionType]v1.ConditionStatus
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

func (c *nodeClient) NodeToCapacityMap() map[string]v1.ResourceList {
	if !c.captureNodeLevelInfo {
		c.logger.Warn("trying to access node level info when captureNodeLevelInfo is not set, will return empty data")
	}
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeToCapacityMap
}

func (c *nodeClient) NodeToAllocatableMap() map[string]v1.ResourceList {
	if !c.captureNodeLevelInfo {
		c.logger.Warn("trying to access node level info when captureNodeLevelInfo is not set, will return empty data")
	}
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeToAllocatableMap
}

func (c *nodeClient) NodeToConditionsMap() map[string]map[v1.NodeConditionType]v1.ConditionStatus {
	if !c.captureNodeLevelInfo {
		c.logger.Warn("trying to access node level info when captureNodeLevelInfo is not set, will return empty data")
	}
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeToConditionsMap
}

func (c *nodeClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	objsList := c.store.List()

	clusterFailedNodeCountNew := 0
	clusterNodeCountNew := 0
	nodeToCapacityMap := make(map[string]v1.ResourceList)
	nodeToAllocatableMap := make(map[string]v1.ResourceList)
	nodeToConditionsMap := make(map[string]map[v1.NodeConditionType]v1.ConditionStatus)

	for _, obj := range objsList {
		node := obj.(*nodeInfo)

		if c.captureNodeLevelInfo {
			nodeToCapacityMap[node.name] = node.capacity
			nodeToAllocatableMap[node.name] = node.allocatable
			conditionsMap := make(map[v1.NodeConditionType]v1.ConditionStatus)
			for _, condition := range node.conditions {
				conditionsMap[condition.Type] = condition.Status
			}
			nodeToConditionsMap[node.name] = conditionsMap
		}
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
	c.nodeToCapacityMap = nodeToCapacityMap
	c.nodeToAllocatableMap = nodeToAllocatableMap
	c.nodeToConditionsMap = nodeToConditionsMap
}

func newNodeClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...nodeClientOption) *nodeClient {
	c := &nodeClient{
		stopChan: make(chan struct{}),
		logger:   logger,
	}

	for _, option := range options {
		option(c)
	}

	c.store = NewObjStore(transformFuncNode, logger)

	lw := c.createNodeListWatch(clientSet)
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

func transformFuncNode(obj interface{}) (interface{}, error) {
	node, ok := obj.(*v1.Node)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not Node type", obj)
	}
	info := new(nodeInfo)
	info.name = node.Name
	info.capacity = node.Status.Capacity
	info.allocatable = node.Status.Allocatable
	info.conditions = []*NodeCondition{}
	for _, condition := range node.Status.Conditions {
		info.conditions = append(info.conditions, &NodeCondition{
			Type:   condition.Type,
			Status: condition.Status,
		})
	}
	return info, nil
}

func (c *nodeClient) createNodeListWatch(client kubernetes.Interface) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			if c.nodeSelector != nil {
				opts.FieldSelector = c.nodeSelector.String()
			}
			return client.CoreV1().Nodes().List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			if c.nodeSelector != nil {
				opts.FieldSelector = c.nodeSelector.String()
			}
			return client.CoreV1().Nodes().Watch(ctx, opts)
		},
	}
}
