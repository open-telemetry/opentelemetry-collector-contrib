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
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

// FakeClient is used as a replacement for WatchClient in test cases.
type FakeClient struct {
	Pods     map[string]*Pod
	Rules    ExtractionRules
	Filters  Filters
	Informer cache.SharedInformer
	StopCh   chan struct{}
}

// NewFakeClient instantiates a new FakeClient object and satisfies the ClientProvider type
func NewFakeClient(_ *zap.Logger, apiCfg k8sconfig.APIConfig, rules ExtractionRules, filters Filters, _ APIClientsetProvider, _ InformerProvider) (Client, error) {
	cs, err := newFakeAPIClientset(apiCfg)
	if err != nil {
		return nil, err
	}

	labelSelector, fieldSelector, err := selectorsFromFilters(filters)
	if err != nil {
		return nil, err
	}
	return &FakeClient{
		Pods:     map[string]*Pod{},
		Rules:    rules,
		Filters:  filters,
		Informer: newFakeInformer(cs, "", labelSelector, fieldSelector),
		StopCh:   make(chan struct{}),
	}, nil
}

// GetPodByIP looks up FakeClient.Pods map by the provided string.
func (f *FakeClient) GetPodByIP(ip string) (*Pod, bool) {
	p, ok := f.Pods[ip]
	return p, ok
}

// Start is a noop for FakeClient.
func (f *FakeClient) Start() {
	if f.Informer != nil {
		f.Informer.Run(f.StopCh)
	}
}

// Stop is a noop for FakeClient.
func (f *FakeClient) Stop() {
	close(f.StopCh)
}

func newFakeAPIClientset(_ k8sconfig.APIConfig) (*kubernetes.Clientset, error) {
	return &kubernetes.Clientset{}, nil
}
