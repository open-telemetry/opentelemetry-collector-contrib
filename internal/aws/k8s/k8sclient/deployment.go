// Copyright  OpenTelemetry Authors
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

type DeploymentClient interface {
	// DeploymentInfos contains the information about each deployment in the cluster
	DeploymentInfos() []*DeploymentInfo
}

type noOpDeploymentClient struct {
}

func (nd *noOpDeploymentClient) DeploymentInfos() []*DeploymentInfo {
	return []*DeploymentInfo{}
}

func (nd *noOpDeploymentClient) shutdown() {
}

type deploymentClientOption func(*deploymentClient)

func deploymentSyncCheckerOption(checker initialSyncChecker) deploymentClientOption {
	return func(d *deploymentClient) {
		d.syncChecker = checker
	}
}

type deploymentClient struct {
	stopChan chan struct{}
	stopped  bool

	store *ObjStore

	syncChecker initialSyncChecker

	mu              sync.RWMutex
	deploymentInfos []*DeploymentInfo
}

func (d *deploymentClient) refresh() {
	d.mu.Lock()
	defer d.mu.Unlock()

	var deploymentInfos []*DeploymentInfo
	objsList := d.store.List()
	for _, obj := range objsList {
		deployment, ok := obj.(*DeploymentInfo)
		if !ok {
			continue
		}
		deploymentInfos = append(deploymentInfos, deployment)
	}

	d.deploymentInfos = deploymentInfos
}

func (d *deploymentClient) DeploymentInfos() []*DeploymentInfo {
	if d.store.GetResetRefreshStatus() {
		d.refresh()
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.deploymentInfos
}

func newDeploymentClient(clientSet kubernetes.Interface, logger *zap.Logger, options ...deploymentClientOption) (*deploymentClient, error) {
	d := &deploymentClient{
		stopChan: make(chan struct{}),
	}

	for _, option := range options {
		option(d)
	}

	ctx := context.Background()
	if _, err := clientSet.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("cannot list Deployments. err: %w", err)
	}

	d.store = NewObjStore(transformFuncDeployment, logger)
	lw := createDeploymentListWatch(clientSet, metav1.NamespaceAll)
	reflector := cache.NewReflector(lw, &appsv1.Deployment{}, d.store, 0)

	go reflector.Run(d.stopChan)

	if d.syncChecker != nil {
		// check the init sync for potential connection issue
		d.syncChecker.Check(reflector, "Deployment initial sync timeout")
	}

	return d, nil
}

func (d *deploymentClient) shutdown() {
	close(d.stopChan)
	d.stopped = true
}

func transformFuncDeployment(obj interface{}) (interface{}, error) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("input obj %v is not Deployment type", obj)
	}
	info := new(DeploymentInfo)
	info.Name = deployment.Name
	info.Namespace = deployment.Namespace
	info.Spec = &DeploymentSpec{
		Replicas: uint32(*deployment.Spec.Replicas),
	}
	info.Status = &DeploymentStatus{
		Replicas:            uint32(deployment.Status.Replicas),
		AvailableReplicas:   uint32(deployment.Status.AvailableReplicas),
		UnavailableReplicas: uint32(deployment.Status.UnavailableReplicas),
	}
	return info, nil
}

func createDeploymentListWatch(client kubernetes.Interface, ns string) cache.ListerWatcher {
	ctx := context.Background()
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.AppsV1().Deployments(ns).List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.AppsV1().Deployments(ns).Watch(ctx, opts)
		},
	}
}
