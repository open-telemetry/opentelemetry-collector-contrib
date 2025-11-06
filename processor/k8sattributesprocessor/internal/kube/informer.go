// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"

	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const kubeSystemNamespace = "kube-system"

// InformerProvider defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
type InformerProvider func(
	client kubernetes.Interface,
	namespace string,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
) cache.SharedInformer

// InformerProviderNamespace defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching namespace objects.
type InformerProviderNamespace func(
	client kubernetes.Interface,
) cache.SharedInformer

// InformerProviderNode defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client for fetching node objects.
type InformerProviderNode func(
	client kubernetes.Interface,
) cache.SharedInformer

// InformerProviderWorkload defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
// It's used for high-level workloads such as ReplicaSets, Deployments, DaemonSets, StatefulSets or Jobs
type InformerProviderWorkload func(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer

func newSharedInformer(
	client kubernetes.Interface,
	namespace string,
	ls labels.Selector,
	fs fields.Selector,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  informerListFuncWithSelectors(client, namespace, ls, fs),
			WatchFunc: informerWatchFuncWithSelectors(client, namespace, ls, fs),
		},
		&api_v1.Pod{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func informerListFuncWithSelectors(client kubernetes.Interface, namespace string, ls labels.Selector, fs fields.Selector) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		opts.LabelSelector = ls.String()
		opts.FieldSelector = fs.String()
		return client.CoreV1().Pods(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
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
	return informer
}

func newNamespaceSharedInformer(
	client kubernetes.Interface,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  namespaceInformerListFunc(client),
			WatchFunc: namespaceInformerWatchFunc(client),
		},
		&api_v1.Namespace{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func namespaceInformerListFunc(client kubernetes.Interface) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Namespaces().List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func namespaceInformerWatchFunc(client kubernetes.Interface) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Namespaces().Watch(context.Background(), opts)
	}
}

func newReplicaSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  replicasetListFuncWithSelectors(client, namespace),
			WatchFunc: replicasetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.ReplicaSet{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func replicasetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().ReplicaSets(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func replicasetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().ReplicaSets(namespace).Watch(context.Background(), opts)
	}
}

func newDeploymentSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  deploymentListFuncWithSelectors(client, namespace),
			WatchFunc: deploymentWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.Deployment{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func deploymentListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().Deployments(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func deploymentWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().Deployments(namespace).Watch(context.Background(), opts)
	}
}

func newStatefulSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  statefulsetListFuncWithSelectors(client, namespace),
			WatchFunc: statefulsetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.StatefulSet{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func statefulsetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().StatefulSets(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func statefulsetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().StatefulSets(namespace).Watch(context.Background(), opts)
	}
}

func newDaemonSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  daemonsetListFuncWithSelectors(client, namespace),
			WatchFunc: daemonsetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.DaemonSet{},
		watchSyncPeriod,
	)
	return informer
}

func newJobSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc:  jobListFuncWithSelectors(client, namespace),
			WatchFunc: jobWatchFuncWithSelectors(client, namespace),
		},
		&batch_v1.Job{},
		watchSyncPeriod,
	)
	return informer
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func jobListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.BatchV1().Jobs(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func jobWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.BatchV1().Jobs(namespace).Watch(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func daemonsetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListFunc {
	return func(opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().DaemonSets(namespace).List(context.Background(), opts)
	}
}

//nolint:staticcheck // SA1019 TODO: resolve as part of https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43891
func daemonsetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFunc {
	return func(opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().DaemonSets(namespace).Watch(context.Background(), opts)
	}
}
