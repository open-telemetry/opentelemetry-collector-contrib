// Copyright 2019 Omnition Authors
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

package kube

import (
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type fakeInformer struct {
	fakeController

	namespace     string
	labelSelector labels.Selector
	fieldSelector fields.Selector
}

func newFakeInformer(
	client *kubernetes.Clientset,
	namespace string,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
) cache.SharedInformer {
	return fakeInformer{
		namespace:     namespace,
		labelSelector: labelSelector,
		fieldSelector: fieldSelector,
	}
}

func (f fakeInformer) AddEventHandler(handler cache.ResourceEventHandler) {}

func (f fakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, period time.Duration) {
}

func (f fakeInformer) GetStore() cache.Store {
	return cache.NewStore(func(obj interface{}) (string, error) { return "", nil })
}

func (f fakeInformer) GetController() cache.Controller {
	return f.fakeController
}

type fakeController struct{}

func (c fakeController) HasSynced() bool {
	return true
}

func (c fakeController) Run(stopCh <-chan struct{}) {}

func (c fakeController) LastSyncResourceVersion() string {
	return ""
}
