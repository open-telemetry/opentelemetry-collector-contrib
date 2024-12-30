// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
	"time"

	apps_v1 "k8s.io/api/apps/v1"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const kubeSystemNamespace = "kube-system"

// InformerProviders holds factory functions for informers needed by the kube client. The intent is to facilitate
// informer sharing by letting the processor factory pass in custom implementations of these factories.
type InformerProviders struct {
	PodInformerProvider        InformerProvider
	NamespaceInformerProvider  InformerProviderNamespace
	ReplicaSetInformerProvider InformerProviderReplicaSet
	NodeInformerProvider       InformerProviderNode
}

// NewDefaultInformerProviders returns informer providers using the given client. These are not shared, and exist
// to provide a simple default implementation.
func NewDefaultInformerProviders(client kubernetes.Interface) *InformerProviders {
	podInformerProvider := func(
		namespace string,
		labelSelector labels.Selector,
		fieldSelector fields.Selector,
		transformFunc cache.TransformFunc,
		stopCh <-chan struct{},
	) (cache.SharedInformer, error) {
		return newSharedInformer(client, namespace, labelSelector, fieldSelector, transformFunc, stopCh)
	}

	namespaceInformerProvider := func(
		fs fields.Selector,
		stopCh <-chan struct{},
	) (cache.SharedInformer, error) {
		return newNamespaceSharedInformer(client, fs, stopCh)
	}

	replicaSetInformerProvider := func(
		namespace string,
		transformFunc cache.TransformFunc,
		stopCh <-chan struct{},
	) (cache.SharedInformer, error) {
		return newReplicaSetSharedInformer(client, namespace, transformFunc, stopCh)
	}

	nodeInformerProvider := func(
		nodeName string,
		watchSyncPeriod time.Duration,
		stopCh <-chan struct{},
	) (cache.SharedInformer, error) {
		return newNodeSharedInformer(client, nodeName, watchSyncPeriod, stopCh)
	}

	return &InformerProviders{
		PodInformerProvider:        podInformerProvider,
		NamespaceInformerProvider:  namespaceInformerProvider,
		ReplicaSetInformerProvider: replicaSetInformerProvider,
		NodeInformerProvider:       nodeInformerProvider,
	}
}

// InformerProvider defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
type InformerProvider func(
	namespace string,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
	transformFunc cache.TransformFunc,
	stopCh <-chan struct{},
) (cache.SharedInformer, error)

// InformerProviderNamespace defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching namespace objects.
type InformerProviderNamespace func(
	fieldSelector fields.Selector,
	stopCh <-chan struct{},
) (cache.SharedInformer, error)

// InformerProviderNode defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching node objects.
type InformerProviderNode func(
	nodeName string,
	watchSyncPeriod time.Duration,
	stopCh <-chan struct{},
) (cache.SharedInformer, error)

// InformerProviderReplicaSet defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
type InformerProviderReplicaSet func(
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh <-chan struct{},
) (cache.SharedInformer, error)

func newSharedInformer(
	client kubernetes.Interface,
	namespace string,
	ls labels.Selector,
	fs fields.Selector,
	transformFunc cache.TransformFunc,
	stopCh <-chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  informerListFuncWithSelectors(client, namespace, ls, fs),
			WatchFunc: informerWatchFuncWithSelectors(client, namespace, ls, fs),
		},
		&api_v1.Pod{},
		watchSyncPeriod,
	)
	if transformFunc != nil {
		err := informer.SetTransform(transformFunc)
		if err != nil {
			return nil, err
		}
	}
	go informer.Run(stopCh)
	return informer, nil
}

func informerListFuncWithSelectors(client kubernetes.Interface, namespace string, ls labels.Selector, fs fields.Selector) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		opts.LabelSelector = ls.String()
		opts.FieldSelector = fs.String()
		return client.CoreV1().Pods(namespace).List(context.Background(), opts)
	}
}

func informerWatchFuncWithSelectors(client kubernetes.Interface, namespace string, ls labels.Selector, fs fields.Selector) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		opts.LabelSelector = ls.String()
		opts.FieldSelector = fs.String()
		return client.CoreV1().Pods(namespace).Watch(context.Background(), opts)
	}
}

func newNamespaceSharedInformer(
	client kubernetes.Interface,
	fs fields.Selector,
	stopCh <-chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  namespaceInformerListFunc(client, fs),
			WatchFunc: namespaceInformerWatchFunc(client, fs),
		},
		&api_v1.Namespace{},
		watchSyncPeriod,
	)
	go informer.Run(stopCh)
	return informer, nil
}

func namespaceInformerListFunc(client kubernetes.Interface, fs fields.Selector) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		if fs != nil {
			opts.FieldSelector = fs.String()
		}
		return client.CoreV1().Namespaces().List(context.Background(), opts)
	}
}

func namespaceInformerWatchFunc(client kubernetes.Interface, fs fields.Selector) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		if fs != nil {
			opts.FieldSelector = fs.String()
		}
		return client.CoreV1().Namespaces().Watch(context.Background(), opts)
	}
}

func newReplicaSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh <-chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  replicasetListFuncWithSelectors(client, namespace),
			WatchFunc: replicasetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.ReplicaSet{},
		watchSyncPeriod,
	)
	if transformFunc != nil {
		err := informer.SetTransform(transformFunc)
		if err != nil {
			return nil, err
		}
	}
	go informer.Run(stopCh)
	return informer, nil
}

func replicasetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().ReplicaSets(namespace).List(context.Background(), opts)
	}
}

func replicasetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().ReplicaSets(namespace).Watch(context.Background(), opts)
	}
}

func newNodeSharedInformer(
	client kubernetes.Interface,
	nodeName string,
	watchSyncPeriod time.Duration,
	stopCh <-chan struct{},
) (cache.SharedInformer, error) {
	informer := k8sconfig.NewNodeSharedInformer(client, nodeName, watchSyncPeriod)
	go informer.Run(stopCh)
	return informer, nil
}
