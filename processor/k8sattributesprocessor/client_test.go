// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

// fakeClient is used as a replacement for WatchClient in test cases.
type fakeClient struct {
	Pods               map[kube.PodIdentifier]*kube.Pod
	Rules              kube.ExtractionRules
	Filters            kube.Filters
	Associations       []kube.Association
	Informer           cache.SharedInformer
	NamespaceInformer  cache.SharedInformer
	ReplicaSetInformer cache.SharedInformer
	NodeInformer       cache.SharedInformer
	Namespaces         map[string]*kube.Namespace
	Nodes              map[string]*kube.Node
	StopCh             chan struct{}
}

func selectors() (labels.Selector, fields.Selector) {
	var selectors []fields.Selector
	return labels.Everything(), fields.AndSelectors(selectors...)
}

// newFakeClient instantiates a new FakeClient object and satisfies the ClientProvider type
func newFakeClient(
	_ component.TelemetrySettings,
	rules kube.ExtractionRules,
	filters kube.Filters,
	associations []kube.Association,
	_ kube.Excludes,
	_ *kube.InformerProviders,
	_ bool,
	_ time.Duration,
) (kube.Client, error) {
	ls, fs := selectors()
	closeCh := make(chan struct{})
	informer, err := kube.NewFakeInformer("", ls, fs, nil, closeCh)
	if err != nil {
		return nil, err
	}
	nsInformer, err := kube.NewFakeInformer("", ls, fs, nil, closeCh)
	if err != nil {
		return nil, err
	}
	nodeInformer, err := kube.NewFakeInformer("", ls, fs, nil, closeCh)
	if err != nil {
		return nil, err
	}
	rsInformer, err := kube.NewFakeInformer("", ls, fs, nil, closeCh)
	if err != nil {
		return nil, err
	}
	return &fakeClient{
		Pods:               map[kube.PodIdentifier]*kube.Pod{},
		Rules:              rules,
		Filters:            filters,
		Associations:       associations,
		Informer:           informer,
		NamespaceInformer:  nsInformer,
		NodeInformer:       nodeInformer,
		ReplicaSetInformer: rsInformer,
		StopCh:             make(chan struct{}),
	}, nil
}

// GetPod looks up FakeClient.Pods map by the provided string,
// which might represent either IP address or Pod UID.
func (f *fakeClient) GetPod(identifier kube.PodIdentifier) (*kube.Pod, bool) {
	p, ok := f.Pods[identifier]
	return p, ok
}

func (f *fakeClient) GetNamespace(namespace string) (*kube.Namespace, bool) {
	ns, ok := f.Namespaces[namespace]
	return ns, ok
}

func (f *fakeClient) GetNode(nodeName string) (*kube.Node, bool) {
	node, ok := f.Nodes[nodeName]
	return node, ok
}

// Start is a noop for FakeClient.
func (f *fakeClient) Start() error {
	if f.Informer != nil {
		go f.Informer.Run(f.StopCh)
	}
	return nil
}

// Stop is a noop for FakeClient.
func (f *fakeClient) Stop() {
	close(f.StopCh)
}
