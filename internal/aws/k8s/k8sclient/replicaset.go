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
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	deployment = "Deployment"
)

type ReplicaSetClient interface {
	// Get the mapping between replica set and deployment
	ReplicaSetToDeployment() map[string]string
}

type noOpReplicaSetClient struct {
}

func (nc *noOpReplicaSetClient) ReplicaSetToDeployment() map[string]string {
	return map[string]string{}
}

func (nc *noOpReplicaSetClient) shutdown() {
}

type replicaSetClientOption func(*replicaSetClient)

func replicaSetSyncCheckerOption(checker initialSyncChecker) replicaSetClientOption {
	return func(r *replicaSetClient) {
		r.syncChecker = checker
	}
}

type replicaSetClient struct {
	stopChan chan struct{}
	store    *ObjStore

	stopped     bool
	syncChecker initialSyncChecker

	mu                        sync.RWMutex
	cachedReplicaSetMap       map[string]time.Time
	replicaSetToDeploymentMap map[string]string
}

func (c *replicaSetClient) ReplicaSetToDeployment() map[string]string {
	if c.store.GetResetRefreshStatus() {
		c.refresh()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.replicaSetToDeploymentMap
}

func (c *replicaSetClient) refresh() {
	c.mu.Lock()
	defer c.mu.Unlock()

	objsList := c.store.List()

	tmpMap := make(map[string]string)
	for _, obj := range objsList {
		replicaSet := obj.(*replicaSetInfo)
	ownerLoop:
		for _, owner := range replicaSet.owners {
			if owner.kind == deployment && owner.name != "" {
				tmpMap[replicaSet.name] = owner.name
				break ownerLoop
			}
		}
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

func newReplicaSetClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...replicaSetClientOption) (*replicaSetClient, error) {
	c := &replicaSetClient{
		stopChan:                  make(chan struct{}),
		cachedReplicaSetMap:       make(map[string]time.Time),
		replicaSetToDeploymentMap: make(map[string]string),
	}

	for _, option := range options {
		option(c)
	}

	ctx := context.Background()
	if _, err := clientSet.AppsV1().ReplicaSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list ReplicaSet. err: %w", err)
	}

	c.stopChan = make(chan struct{})

	c.store = NewObjStore(transformFuncReplicaSet, logger)

	lw := createReplicaSetListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &appsv1.ReplicaSet{}, c.store, 0)
	go reflector.Run(c.stopChan)

	if c.syncChecker != nil {
		// check the init sync for potential connection issue
		c.syncChecker.Check(reflector, "ReplicaSet initial sync timeout")
	}

	return c, nil
}

func (c *replicaSetClient) shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.stopChan)
	c.stopped = true
}

func transformFuncReplicaSet(obj interface{}) (interface{}, error) {
	replicaSet, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not ReplicaSet type", obj)
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
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.AppsV1().ReplicaSets(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.AppsV1().ReplicaSets(ns).Watch(ctx, opts)
		},
	}
}
