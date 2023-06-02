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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type DaemonSetClient interface {
	// DaemonSetInfos contains the information about each daemon set in the cluster
	DaemonSetInfos() []*DaemonSetInfo
}

type noOpDaemonSetClient struct {
}

func (nd *noOpDaemonSetClient) DaemonSetInfos() []*DaemonSetInfo {
	return []*DaemonSetInfo{}
}

func (nd *noOpDaemonSetClient) shutdown() {
}

type daemonSetClientOption func(*daemonSetClient)

func daemonSetSyncCheckerOption(checker initialSyncChecker) daemonSetClientOption {
	return func(d *daemonSetClient) {
		d.syncChecker = checker
	}
}

type daemonSetClient struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu             sync.RWMutex
	daemonSetInfos []*DaemonSetInfo
}

func (d *daemonSetClient) refresh() {
	d.mu.Lock()
	defer d.mu.Unlock()

	var daemonSetInfos []*DaemonSetInfo
	objsList := d.store.List()
	for _, obj := range objsList {
		daemonSet, ok := obj.(*DaemonSetInfo)
		if !ok {
			continue
		}
		daemonSetInfos = append(daemonSetInfos, daemonSet)
	}

	d.daemonSetInfos = daemonSetInfos
}

func (d *daemonSetClient) DaemonSetInfos() []*DaemonSetInfo {
	if d.store.GetResetRefreshStatus() {
		d.refresh()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.daemonSetInfos
}

func newDaemonSetClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...daemonSetClientOption) (*daemonSetClient, error) {
	d := &daemonSetClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(d)
	}

	ctx := context.Background()
	if _, err := clientSet.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list DaemonSets. err: %w", err)
	}

	d.store = NewObjStore(transformFuncDaemonSet, logger)
	lw := createDaemonSetListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &appsv1.DaemonSet{}, d.store, 0)

	go reflector.Run(d.stopChan)

	if d.syncChecker != nil {
		// check the init sync for potential connection issue
		d.syncChecker.Check(reflector, "DaemonSet initial sync timeout")
	}

	return d, nil
}

func (d *daemonSetClient) shutdown() {
	close(d.stopChan)
	d.stopped = true
}

func transformFuncDaemonSet(obj interface{}) (interface{}, error) {
	daemonSet, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not DaemonSet type", obj)
	}
	info := new(DaemonSetInfo)
	info.Name = daemonSet.Name
	info.Namespace = daemonSet.Namespace
	info.Status = &DaemonSetStatus{
		NumberAvailable:        uint32(daemonSet.Status.NumberAvailable),
		NumberUnavailable:      uint32(daemonSet.Status.NumberUnavailable),
		DesiredNumberScheduled: uint32(daemonSet.Status.DesiredNumberScheduled),
		CurrentNumberScheduled: uint32(daemonSet.Status.CurrentNumberScheduled),
	}
	return info, nil
}

func createDaemonSetListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.AppsV1().DaemonSets(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.AppsV1().DaemonSets(ns).Watch(ctx, opts)
		},
	}
}
