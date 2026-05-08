// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
	"time"

	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
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
	client metadata.Interface,
) cache.SharedInformer

// InformerProviderWorkload defines a function type that returns a new SharedInformer. It is used to
// allow passing custom shared informers to the watch client.
// It's used for high-level workloads such as ReplicaSets, Deployments, DaemonSets, StatefulSets or Jobs
type InformerProviderWorkload func(
	client metadata.Interface,
	namespace string,
) cache.SharedInformer

func newSharedInformer(
	client kubernetes.Interface,
	namespace string,
	ls labels.Selector,
	fs fields.Selector,
	watchSyncPeriod time.Duration,
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
	client metadata.Interface,
	watchSyncPeriod time.Duration,
) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", kubeSystemNamespace).String()
				return client.Resource(gvr).List(ctx, opts)
			},
			WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", kubeSystemNamespace).String()
				return client.Resource(gvr).Watch(ctx, opts)
			},
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newNamespaceSharedInformer(
	client metadata.Interface,
	watchSyncPeriod time.Duration,
) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, ""),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, ""),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newReplicaSetSharedInformer(client metadata.Interface, namespace string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, namespace),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, namespace),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newDeploymentSharedInformer(client metadata.Interface, namespace string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, namespace),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, namespace),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newStatefulSetSharedInformer(client metadata.Interface, namespace string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, namespace),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, namespace),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newDaemonSetSharedInformer(client metadata.Interface, namespace string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, namespace),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, namespace),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newJobSharedInformer(client metadata.Interface, namespace string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc:  metadataListFunc(client, gvr, namespace),
			WatchFuncWithContext: metadataWatchFunc(client, gvr, namespace),
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func newNodeSharedInformer(client metadata.Interface, nodeName string, watchSyncPeriod time.Duration) cache.SharedInformer {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	return cache.NewSharedInformer(
		&cache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.Resource(gvr).List(ctx, opts)
			},
			WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				if nodeName != "" {
					opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
				}
				return client.Resource(gvr).Watch(ctx, opts)
			},
		},
		&metav1.PartialObjectMetadata{},
		watchSyncPeriod,
	)
}

func metadataListFunc(mc metadata.Interface, gvr schema.GroupVersionResource, namespace string) cache.ListWithContextFunc {
	return func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return mc.Resource(gvr).Namespace(namespace).List(ctx, opts)
	}
}

func metadataWatchFunc(mc metadata.Interface, gvr schema.GroupVersionResource, namespace string) cache.WatchFuncWithContext {
	return func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
		return mc.Resource(gvr).Namespace(namespace).Watch(ctx, opts)
	}
}
