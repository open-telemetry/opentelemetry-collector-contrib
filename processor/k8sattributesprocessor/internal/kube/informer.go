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
			ListWithContextFunc:  informerListFuncWithSelectors(client, namespace, ls, fs),
			WatchFuncWithContext: informerWatchFuncWithSelectors(client, namespace, ls, fs),
		},
		&api_v1.Pod{},
		watchSyncPeriod,
	)
	return informer
}

func informerListFuncWithSelectors(client kubernetes.Interface, namespace string, ls labels.Selector, fs fields.Selector) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		opts.LabelSelector = ls.String()
		opts.FieldSelector = fs.String()
		return client.CoreV1().Pods(namespace).List(ctx, opts)
	}
}

func informerWatchFuncWithSelectors(client kubernetes.Interface, namespace string, ls labels.Selector, fs fields.Selector) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		opts.LabelSelector = ls.String()
		opts.FieldSelector = fs.String()
		return client.CoreV1().Pods(namespace).Watch(ctx, opts)
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
			ListWithContextFunc:  namespaceInformerListFunc(client),
			WatchFuncWithContext: namespaceInformerWatchFunc(client),
		},
		&api_v1.Namespace{},
		watchSyncPeriod,
	)
	return informer
}

func namespaceInformerListFunc(client kubernetes.Interface) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.CoreV1().Namespaces().List(ctx, opts)
	}
}

func namespaceInformerWatchFunc(client kubernetes.Interface) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.CoreV1().Namespaces().Watch(ctx, opts)
	}
}

func newReplicaSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  replicasetListFuncWithSelectors(client, namespace),
			WatchFuncWithContext: replicasetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.ReplicaSet{},
		watchSyncPeriod,
	)
	return informer
}

func replicasetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().ReplicaSets(namespace).List(ctx, opts)
	}
}

func replicasetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().ReplicaSets(namespace).Watch(ctx, opts)
	}
}

func newDeploymentSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  deploymentListFuncWithSelectors(client, namespace),
			WatchFuncWithContext: deploymentWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.Deployment{},
		watchSyncPeriod,
	)
	return informer
}

func deploymentListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().Deployments(namespace).List(ctx, opts)
	}
}

func deploymentWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().Deployments(namespace).Watch(ctx, opts)
	}
}

func newStatefulSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  statefulsetListFuncWithSelectors(client, namespace),
			WatchFuncWithContext: statefulsetWatchFuncWithSelectors(client, namespace),
		},
		&apps_v1.StatefulSet{},
		watchSyncPeriod,
	)
	return informer
}

func statefulsetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().StatefulSets(namespace).List(ctx, opts)
	}
}

func statefulsetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().StatefulSets(namespace).Watch(ctx, opts)
	}
}

func newDaemonSetSharedInformer(
	client kubernetes.Interface,
	namespace string,
) cache.SharedInformer {
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  daemonsetListFuncWithSelectors(client, namespace),
			WatchFuncWithContext: daemonsetWatchFuncWithSelectors(client, namespace),
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
			ListWithContextFunc:  jobListFuncWithSelectors(client, namespace),
			WatchFuncWithContext: jobWatchFuncWithSelectors(client, namespace),
		},
		&batch_v1.Job{},
		watchSyncPeriod,
	)
	return informer
}

func jobListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.BatchV1().Jobs(namespace).List(ctx, opts)
	}
}

func jobWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.BatchV1().Jobs(namespace).Watch(ctx, opts)
	}
}

func daemonsetListFuncWithSelectors(client kubernetes.Interface, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.AppsV1().DaemonSets(namespace).List(ctx, opts)
	}
}

func daemonsetWatchFuncWithSelectors(client kubernetes.Interface, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return client.AppsV1().DaemonSets(namespace).Watch(ctx, opts)
	}
}
