// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
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
	Deployments        map[string]*kube.Deployment
	StatefulSets       map[string]*kube.StatefulSet
	DaemonSets         map[string]*kube.DaemonSet
	ReplicaSets        map[string]*kube.ReplicaSet
	Jobs               map[string]*kube.Job
	StopCh             chan struct{}
	stopOnce           sync.Once
	stopWg             sync.WaitGroup
}

func selectors() (labels.Selector, fields.Selector) {
	var selectors []fields.Selector
	return labels.Everything(), fields.AndSelectors(selectors...)
}

// newFakeClient instantiates a new FakeClient object and satisfies the ClientProvider type
func newFakeClient(_ component.TelemetrySettings, _ k8sconfig.APIConfig, rules kube.ExtractionRules, filters kube.Filters, associations []kube.Association, _ kube.Excludes, _ kube.APIClientsetProvider, _ kube.InformersFactoryList, _ bool, _ time.Duration) (kube.Client, error) {
	cs := fake.NewClientset()

	ls, fs := selectors()
	return &fakeClient{
		Pods:               map[kube.PodIdentifier]*kube.Pod{},
		Rules:              rules,
		Filters:            filters,
		Associations:       associations,
		Informer:           kube.NewFakeInformer(cs, "", ls, fs),
		NamespaceInformer:  kube.NewFakeInformer(cs, "", ls, fs),
		NodeInformer:       kube.NewFakeInformer(cs, "", ls, fs),
		ReplicaSetInformer: kube.NewFakeInformer(cs, "", ls, fs),
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

func (f *fakeClient) GetDeployment(deploymentUID string) (*kube.Deployment, bool) {
	d, ok := f.Deployments[deploymentUID]
	return d, ok
}

func (f *fakeClient) GetStatefulSet(statefulsetUID string) (*kube.StatefulSet, bool) {
	s, ok := f.StatefulSets[statefulsetUID]
	return s, ok
}

func (f *fakeClient) GetDaemonSet(daemonsetUID string) (*kube.DaemonSet, bool) {
	s, ok := f.DaemonSets[daemonsetUID]
	return s, ok
}

func (f *fakeClient) GetReplicaSet(replicaSetUID string) (*kube.ReplicaSet, bool) {
	rs, ok := f.ReplicaSets[replicaSetUID]
	return rs, ok
}

func (f *fakeClient) GetJob(jobUID string) (*kube.Job, bool) {
	j, ok := f.Jobs[jobUID]
	return j, ok
}

// Start is a noop for FakeClient.
func (f *fakeClient) Start() error {
	startInformer := func(informer cache.SharedInformer) {
		if informer != nil {
			f.stopWg.Go(func() {
				informer.Run(f.StopCh)
			})
		}
	}

	startInformer(f.Informer)
	startInformer(f.NamespaceInformer)
	startInformer(f.NodeInformer)
	startInformer(f.ReplicaSetInformer)

	return nil
}

// Stop is a noop for FakeClient.
func (f *fakeClient) Stop() {
	f.stopOnce.Do(func() {
		if f.StopCh != nil {
			close(f.StopCh)
			f.stopWg.Wait()
		}
	})
}
