// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
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

func (f *FakeInformer) AddEventHandlerWithResyncPeriod(_ cache.ResourceEventHandler, _ time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return f, nil
}

func (f *FakeInformer) AddEventHandlerWithOptions(cache.ResourceEventHandler, cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	return f, nil
}

func (*FakeInformer) RemoveEventHandler(cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (*FakeInformer) IsStopped() bool {
	return false
}

func (*FakeInformer) SetTransform(cache.TransformFunc) error {
	return nil
}

func (*FakeInformer) GetStore() cache.Store {
	return cache.NewStore(func(_ any) (string, error) { return "", nil })
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

func (*FakeNamespaceInformer) AddEventHandler(cache.ResourceEventHandler) {}

func (*FakeNamespaceInformer) AddEventHandlerWithResyncPeriod(cache.ResourceEventHandler, time.Duration) {
}

func (*FakeNamespaceInformer) GetStore() cache.Store {
	return cache.NewStore(func(any) (string, error) { return "", nil })
}

func (f *FakeNamespaceInformer) GetController() cache.Controller {
	return f.FakeController
}

type FakeReplicaSetInformer struct {
	*FakeController
}

func NewFakeReplicaSetInformer(
	_ kubernetes.Interface,
	_ string,
) cache.SharedInformer {
	return &FakeInformer{
		FakeController: &FakeController{},
	}
}

func (*FakeReplicaSetInformer) AddEventHandler(cache.ResourceEventHandler) {}

func (*FakeReplicaSetInformer) AddEventHandlerWithResyncPeriod(cache.ResourceEventHandler, time.Duration) {
}

func (*FakeReplicaSetInformer) SetTransform(cache.TransformFunc) error {
	return nil
}

func (*FakeReplicaSetInformer) GetStore() cache.Store {
	return cache.NewStore(func(any) (string, error) { return "", nil })
}

func (f *FakeReplicaSetInformer) GetController() cache.Controller {
	return f.FakeController
}

type FakeController struct {
	sync.Mutex
	stopped bool
}

func (*FakeController) HasSynced() bool {
	return true
}

func (c *FakeController) Run(stopCh <-chan struct{}) {
	<-stopCh
	c.Lock()
	c.stopped = true
	c.Unlock()
}

func (c *FakeController) RunWithContext(ctx context.Context) {
	c.Run(ctx.Done())
}

func (c *FakeController) HasStopped() bool {
	c.Lock()
	defer c.Unlock()
	return c.stopped
}

func (*FakeController) LastSyncResourceVersion() string {
	return ""
}

func (*FakeInformer) SetWatchErrorHandler(cache.WatchErrorHandler) error {
	return nil
}

func (*FakeInformer) SetWatchErrorHandlerWithContext(cache.WatchErrorHandlerWithContext) error {
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

func (f *NoOpInformer) AddEventHandlerWithResyncPeriod(cache.ResourceEventHandler, time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return f, nil
}

func (f *NoOpInformer) AddEventHandlerWithOptions(cache.ResourceEventHandler, cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	return f, nil
}

func (*NoOpInformer) RemoveEventHandler(cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (*NoOpInformer) SetTransform(cache.TransformFunc) error {
	return nil
}

func (*NoOpInformer) GetStore() cache.Store {
	return cache.NewStore(func(any) (string, error) { return "", nil })
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

func (c *NoOpController) RunWithContext(ctx context.Context) {
	c.Run(ctx.Done())
}

func (c *NoOpController) IsStopped() bool {
	return c.hasStopped
}

func (*NoOpController) HasSynced() bool {
	return true
}

func (*NoOpController) LastSyncResourceVersion() string {
	return ""
}

func (*NoOpController) SetWatchErrorHandler(cache.WatchErrorHandler) error {
	return nil
}

func (*NoOpController) SetWatchErrorHandlerWithContext(cache.WatchErrorHandlerWithContext) error {
	return nil
}
