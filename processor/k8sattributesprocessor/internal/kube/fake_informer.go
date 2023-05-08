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

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type FakeInformer struct {
	*FakeController

	namespace     string
	labelSelector labels.Selector
	fieldSelector fields.Selector
}

func NewFakeInformer(
	_ kubernetes.Interface,
	namespace string,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
) cache.SharedInformer {
	return &FakeInformer{
		FakeController: &FakeController{},
		namespace:      namespace,
		labelSelector:  labelSelector,
		fieldSelector:  fieldSelector,
	}
}

func (f *FakeInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return f.AddEventHandlerWithResyncPeriod(handler, time.Second)
}

func (f *FakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (f *FakeInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (f *FakeInformer) IsStopped() bool {
	return false
}

func (f *FakeInformer) SetTransform(handler cache.TransformFunc) error {
	return nil
}

func (f *FakeInformer) GetStore() cache.Store {
	return cache.NewStore(func(obj interface{}) (string, error) { return "", nil })
}

func (f *FakeInformer) GetController() cache.Controller {
	return f.FakeController
}

type FakeNamespaceInformer struct {
	*FakeController
}

func NewFakeNamespaceInformer(
	_ kubernetes.Interface,
) cache.SharedInformer {
	return &FakeInformer{
		FakeController: &FakeController{},
	}
}

func (f *FakeNamespaceInformer) AddEventHandler(handler cache.ResourceEventHandler) {}

func (f *FakeNamespaceInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, period time.Duration) {
}

func (f *FakeNamespaceInformer) GetStore() cache.Store {
	return cache.NewStore(func(obj interface{}) (string, error) { return "", nil })
}

func (f *FakeNamespaceInformer) GetController() cache.Controller {
	return f.FakeController
}

type FakeController struct {
	sync.Mutex
	stopped bool
}

func (c *FakeController) HasSynced() bool {
	return true
}

func (c *FakeController) Run(stopCh <-chan struct{}) {
	<-stopCh
	c.Lock()
	c.stopped = true
	c.Unlock()
}

func (c *FakeController) HasStopped() bool {
	c.Lock()
	defer c.Unlock()
	return c.stopped
}

func (c *FakeController) LastSyncResourceVersion() string {
	return ""
}

func (f *FakeInformer) SetWatchErrorHandler(cache.WatchErrorHandler) error {
	return nil
}

type NoOpInformer struct {
	*NoOpController
}

func NewNoOpInformer(
	_ kubernetes.Interface,
) cache.SharedInformer {
	return &NoOpInformer{
		NoOpController: &NoOpController{},
	}
}

func (f *NoOpInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return f.AddEventHandlerWithResyncPeriod(handler, time.Second)
}
func (f *NoOpInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (f *NoOpInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (f *NoOpInformer) SetTransform(handler cache.TransformFunc) error {
	return nil
}

func (f *NoOpInformer) GetStore() cache.Store {
	return cache.NewStore(func(obj interface{}) (string, error) { return "", nil })
}

func (f *NoOpInformer) GetController() cache.Controller {
	return f.NoOpController
}

type NoOpController struct {
	hasStopped bool
}

func (c *NoOpController) Run(stopCh <-chan struct{}) {
	go func() {
		<-stopCh
		c.hasStopped = true
	}()
}
func (c *NoOpController) IsStopped() bool {
	return c.hasStopped
}
func (c *NoOpController) HasSynced() bool {
	return true
}

func (c *NoOpController) LastSyncResourceVersion() string {
	return ""
}

func (c *NoOpController) SetWatchErrorHandler(cache.WatchErrorHandler) error {
	return nil
}
