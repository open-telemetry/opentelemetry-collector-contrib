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
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

// FakeClient is used as a replacement for WatchClient in test cases.
type FakeClient struct {
	Pods    map[string]*Pod
	Rules   ExtractionRules
	Filters Filters
}

// NewFakeClient instantiates a new FakeClient object and satisfies the ClientProvider type
func NewFakeClient(logger *zap.Logger, rules ExtractionRules, filters Filters, newClientSet APIClientsetProvider, newInformer InformerProvider, newOwnerProvider OwnerProvider) (Client, error) {
	return &FakeClient{map[string]*Pod{}, rules, filters}, nil
}

// GetPodByIP looks up FakeClient.Pods map by the provided string.
func (f *FakeClient) GetPodByIP(ip string) (*Pod, bool) {
	p, ok := f.Pods[ip]
	return p, ok
}

// Start is a noop for FakeClient.
func (f *FakeClient) Start() {}

// Stop is a noop for FakeClient.
func (f *FakeClient) Stop() {}

func newFakeAPIClientset() (*kubernetes.Clientset, error) {
	return &kubernetes.Clientset{}, nil
}
