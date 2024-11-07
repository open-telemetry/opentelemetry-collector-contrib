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

// InformerProvider defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
type InformerProvider func(
	client kubernetes.Interface,
	namespace string,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error)

// InformerProviderNamespace defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching namespace objects.
type InformerProviderNamespace func(
	client kubernetes.Interface,
	stopCh chan struct{},
) cache.SharedInformer

// InformerProviderNode defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching node objects.
type InformerProviderNode func(
	client kubernetes.Interface,
	nodeName string,
	watchSyncPeriod time.Duration,
	stopCh chan struct{},
) cache.SharedInformer

// InformerProviderWorkload defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
// It's used for high-level workloads such as ReplicaSets, Deployments, DaemonSets, StatefulSets or Jobs
type InformerProviderWorkload func(
	client kubernetes.Interface,
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error)

func newSharedInformer(
	client kubernetes.Interface,
	namespace string,
	ls labels.Selector,
	fs fields.Selector,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  informerListFuncWithSelectors(client, namespace, ls, fs),
			WatchFunc: informerWatchFuncWithSelectors(client, namespace, ls, fs),
		},
		&api_v1.Pod{},
		watchSyncPeriod,
	)
	if err := informer.SetTransform(transformFunc); err != nil {
		return nil, err
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

// newKubeSystemSharedInformer watches only kube-system namespace
func newKubeSystemSharedInformer(
	client kubernetes.Interface,
	stopCh chan struct{},
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", kubeSystemNamespace).String()
				return client.CoreV1().Namespaces().List(context.Background(), opts)
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", kubeSystemNamespace).String()
				return client.CoreV1().Namespaces().Watch(context.Background(), opts)
			},
		},
		&api_v1.Namespace{},
		watchSyncPeriod,
	)
	go informer.Run(stopCh)
	return informer
}

func newNamespaceSharedInformer(
	client kubernetes.Interface,
	stopCh chan struct{},
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  namespaceInformerListFunc(client),
			WatchFunc: namespaceInformerWatchFunc(client),
		},
		&api_v1.Namespace{},
		watchSyncPeriod,
	)
	go informer.Run(stopCh)
	return informer
}

func namespaceInformerListFunc(client kubernetes.Interface) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Namespaces().List(context.Background(), opts)
	}
}

func namespaceInformerWatchFunc(client kubernetes.Interface) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Namespaces().Watch(context.Background(), opts)
	}
}

func newReplicaSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  replicasetListFuncWithSelectors(client, namespace),
			WatchFunc: replicasetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.ReplicaSet{},
		watchSyncPeriod,
	)
	if err := informer.SetTransform(transformFunc); err != nil {
		return nil, err
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

func newDeploymentSharedInformer(
	client kubernetes.Interface,
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  deploymentListFuncWithSelectors(client, namespace),
			WatchFunc: deploymentWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.Deployment{},
		watchSyncPeriod,
	)
	if err := informer.SetTransform(transformFunc); err != nil {
		return nil, err
	}
	go informer.Run(stopCh)
	return informer, nil
}

func deploymentListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().Deployments(namespace).List(context.Background(), opts)
	}
}

func deploymentWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().Deployments(namespace).Watch(context.Background(), opts)
	}
}

func newStatefulSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
	transformFunc cache.TransformFunc,
	stopCh chan struct{},
) (cache.SharedInformer, error) {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  statefulsetListFuncWithSelectors(client, namespace),
			WatchFunc: statefulsetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.StatefulSet{},
		watchSyncPeriod,
	)
	if err := informer.SetTransform(transformFunc); err != nil {
		return nil, err
	}
	go informer.Run(stopCh)
	return informer, nil
}

func statefulsetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().StatefulSets(namespace).List(context.Background(), opts)
	}
}

func statefulsetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().StatefulSets(namespace).Watch(context.Background(), opts)
	}
}

func newNodeSharedInformer(
	client kubernetes.Interface,
	nodeName string,
	watchSyncPeriod time.Duration,
	stopCh chan struct{},
) cache.SharedInformer {
	informer := k8sconfig.NewNodeSharedInformer(client, nodeName, watchSyncPeriod)
	go informer.Run(stopCh)
	return informer
}
