// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8smetadata

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

func init() {
	operator.Register("k8s_metadata_decorator", func() operator.Builder { return NewK8sMetadataDecoratorConfig("") })
}

// NewK8sMetadataDecoratorConfig creates a new k8s metadata decorator config with default values
func NewK8sMetadataDecoratorConfig(operatorID string) *K8sMetadataDecoratorConfig {
	return &K8sMetadataDecoratorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "k8s_metadata_decorator"),
		PodNameField:      entry.NewResourceField("k8s.pod.name"),
		NamespaceField:    entry.NewResourceField("k8s.namespace.name"),
		CacheTTL:          helper.Duration{Duration: 10 * time.Minute},
		Timeout:           helper.Duration{Duration: 10 * time.Second},
		AllowProxy:        false,
	}
}

// K8sMetadataDecoratorConfig is the configuration of k8s_metadata_decorator operator
type K8sMetadataDecoratorConfig struct {
	helper.TransformerConfig `yaml:",inline"`
	PodNameField             entry.Field     `json:"pod_name_field,omitempty"  yaml:"pod_name_field,omitempty"`
	NamespaceField           entry.Field     `json:"namespace_field,omitempty" yaml:"namespace_field,omitempty"`
	CacheTTL                 helper.Duration `json:"cache_ttl,omitempty"       yaml:"cache_ttl,omitempty"`
	Timeout                  helper.Duration `json:"timeout,omitempty"         yaml:"timeout,omitempty"`
	AllowProxy               bool            `json:"allow_proxy,omitempty"     yaml:"allow_proxy,omitempty"`
}

// Build will build a k8s_metadata_decorator operator from the supplied configuration
func (c K8sMetadataDecoratorConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(context)
	if err != nil {
		return nil, errors.Wrap(err, "build transformer")
	}

	op := &K8sMetadataDecorator{
		TransformerOperator: transformer,
		podNameField:        c.PodNameField,
		namespaceField:      c.NamespaceField,
		cacheTTL:            c.CacheTTL.Raw(),
		timeout:             c.Timeout.Raw(),
		allowProxy:          c.AllowProxy,
	}

	return []operator.Operator{op}, nil
}

// K8sMetadataDecorator is an operator for decorating entries with kubernetes metadata
type K8sMetadataDecorator struct {
	helper.TransformerOperator
	podNameField   entry.Field
	namespaceField entry.Field

	client      *corev1.CoreV1Client
	appsClient  *appsv1.AppsV1Client
	batchClient *batchv1.BatchV1Client

	namespaceCache MetadataCache
	podCache       MetadataCache
	cacheTTL       time.Duration
	timeout        time.Duration
	allowProxy     bool
}

// MetadataCacheEntry is an entry in the metadata cache
type MetadataCacheEntry struct {
	ClusterName    string
	UID            string
	ExpirationTime time.Time
	Attributes     map[string]string
	Annotations    map[string]string

	AdditionalResourceValues map[string]string
}

// MetadataCache is a cache of kubernetes metadata
type MetadataCache struct {
	m sync.Map
}

// Load will return an entry stored in the metadata cache
func (m *MetadataCache) Load(key string) (MetadataCacheEntry, bool) {
	entry, ok := m.m.Load(key)
	if ok {
		return entry.(MetadataCacheEntry), ok
	}
	return MetadataCacheEntry{}, ok
}

// Store will store an entry in the metadata cache
func (m *MetadataCache) Store(key string, entry MetadataCacheEntry) {
	m.m.Store(key, entry)
}

// Start will start the k8s_metadata_decorator operator
func (k *K8sMetadataDecorator) Start() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.NewError(
			"agent not in kubernetes cluster",
			"the k8s_metadata_decorator operator only supports running in a pod inside a kubernetes cluster",
		)
	}

	if !k.allowProxy {
		config.Proxy = func(*http.Request) (*url.URL, error) {
			return nil, nil
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "build client set")
	}

	k.client = clientset.CoreV1().(*corev1.CoreV1Client)
	k.appsClient = clientset.AppsV1().(*appsv1.AppsV1Client)
	k.batchClient = clientset.BatchV1().(*batchv1.BatchV1Client)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), k.timeout)
	defer cancel()
	namespaceList, err := k.client.Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "test connection list namespaces")
	}

	if len(namespaceList.Items) == 0 {
		k.Warn("During test connection, namespace list came back empty")
		return nil
	}

	namespaceName := namespaceList.Items[0].ObjectMeta.Name
	_, err = k.client.Pods(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "test connection list pods")
	}

	return nil
}

// Process will process an entry received by the k8s_metadata_decorator operator
func (k *K8sMetadataDecorator) Process(ctx context.Context, entry *entry.Entry) error {
	var podName string
	err := entry.Read(k.podNameField, &podName)
	if err != nil {
		return k.HandleEntryError(ctx, entry, errors.Wrap(err, "find pod name").WithDetails("search_field", k.podNameField.String()))
	}

	var namespace string
	err = entry.Read(k.namespaceField, &namespace)
	if err != nil {
		return k.HandleEntryError(ctx, entry, errors.Wrap(err, "find namespace").WithDetails("search_field", k.podNameField.String()))
	}

	nsMeta, err := k.getNamespaceMetadata(ctx, namespace)
	if err != nil {
		return k.HandleEntryError(ctx, entry, err)
	}
	k.decorateEntryWithNamespaceMetadata(nsMeta, entry)

	podMeta, err := k.getPodMetadata(ctx, namespace, podName)
	if err != nil {
		return k.HandleEntryError(ctx, entry, err)
	}
	k.decorateEntryWithPodMetadata(podMeta, entry)

	k.Write(ctx, entry)
	return nil
}

func (k *K8sMetadataDecorator) getNamespaceMetadata(ctx context.Context, namespace string) (MetadataCacheEntry, error) {
	cacheEntry, ok := k.namespaceCache.Load(namespace)

	var err error
	if !ok || cacheEntry.ExpirationTime.Before(time.Now()) {
		cacheEntry, err = k.refreshNamespaceMetadata(ctx, namespace)
	}

	return cacheEntry, err
}

func (k *K8sMetadataDecorator) getPodMetadata(ctx context.Context, namespace, podName string) (MetadataCacheEntry, error) {
	key := namespace + ":" + podName
	cacheEntry, ok := k.podCache.Load(key)

	var err error
	if !ok || cacheEntry.ExpirationTime.Before(time.Now()) {
		cacheEntry, err = k.refreshPodMetadata(ctx, namespace, podName)
	}

	return cacheEntry, err
}

func (k *K8sMetadataDecorator) refreshNamespaceMetadata(ctx context.Context, namespace string) (MetadataCacheEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()

	// Query the API
	namespaceResponse, err := k.client.Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		// Add an empty entry to the cache so we don't continuously retry
		cacheEntry := MetadataCacheEntry{ExpirationTime: time.Now().Add(10 * time.Second)}
		k.namespaceCache.Store(namespace, cacheEntry)
		return cacheEntry, errors.Wrap(err, "get namespace metadata").WithDetails("namespace", namespace).WithDetails("retry_after", "10s")
	}

	// Cache the results
	cacheEntry := MetadataCacheEntry{
		ClusterName:    namespaceResponse.ClusterName,
		ExpirationTime: time.Now().Add(k.cacheTTL),
		UID:            string(namespaceResponse.UID),
		Attributes:     namespaceResponse.Labels,
		Annotations:    namespaceResponse.Annotations,
	}
	k.namespaceCache.Store(namespace, cacheEntry)

	return cacheEntry, nil
}

func (k *K8sMetadataDecorator) refreshPodMetadata(ctx context.Context, namespace, podName string) (MetadataCacheEntry, error) {
	key := namespace + ":" + podName

	subCtx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()

	// Query the API
	podResponse, err := k.client.Pods(namespace).Get(subCtx, podName, metav1.GetOptions{})
	if err != nil {
		// Add an empty entry to the cache so we don't continuously retry
		cacheEntry := MetadataCacheEntry{ExpirationTime: time.Now().Add(10 * time.Second)}
		k.podCache.Store(key, cacheEntry)

		return cacheEntry, errors.Wrap(err, "get pod metadata").WithDetails(
			"namespace", namespace,
			"pod_name", podName,
			"retry_after", "10s",
		)
	}

	// Create the cache entry
	cacheEntry := MetadataCacheEntry{
		ClusterName:    podResponse.ClusterName,
		UID:            string(podResponse.UID),
		ExpirationTime: time.Now().Add(k.cacheTTL),
		Attributes:     podResponse.Labels,
		Annotations:    podResponse.Annotations,
		AdditionalResourceValues: map[string]string{
			"k8s.replicaset.name":            findNameOfKind(podResponse.OwnerReferences, "ReplicaSet"),
			"k8s.daemonset.name":             findNameOfKind(podResponse.OwnerReferences, "DaemonSet"),
			"k8s.statefulset.name":           findNameOfKind(podResponse.OwnerReferences, "StatefulSet"),
			"k8s.job.name":                   findNameOfKind(podResponse.OwnerReferences, "Job"),
			"k8s.replicationcontroller.name": findNameOfKind(podResponse.OwnerReferences, "ReplicationController"),
			"k8s.service.name":               findNameOfKind(podResponse.OwnerReferences, "ServiceName"),
		},
	}

	// Add deployment if it exists
	rsName := cacheEntry.AdditionalResourceValues["k8s.replicaset.name"]
	if rsName != "" {
		subCtx, cancel := context.WithTimeout(ctx, k.timeout)
		defer cancel()
		replicaSetResponse, err := k.appsClient.ReplicaSets(namespace).Get(subCtx, rsName, metav1.GetOptions{})
		if err != nil {
			k.Errorw("Failed to get replica set metadata", "error", err)
		} else {
			cacheEntry.AdditionalResourceValues["k8s.deployment.name"] = findNameOfKind(replicaSetResponse.OwnerReferences, "Deployment")
		}
	}

	// Add cron job name if it exists
	jobName := cacheEntry.AdditionalResourceValues["k8s.job.name"]
	if jobName != "" {
		subCtx, cancel := context.WithTimeout(ctx, k.timeout)
		defer cancel()
		replicaSetResponse, err := k.batchClient.Jobs(namespace).Get(subCtx, jobName, metav1.GetOptions{})
		if err != nil {
			k.Errorw("Failed to get replica set metadata", "error", err)
		} else {
			cacheEntry.AdditionalResourceValues["k8s.cronjob.name"] = findNameOfKind(replicaSetResponse.OwnerReferences, "CronJob")
		}
	}

	k.podCache.Store(key, cacheEntry)

	return cacheEntry, nil
}

func (k *K8sMetadataDecorator) decorateEntryWithNamespaceMetadata(nsMeta MetadataCacheEntry, entry *entry.Entry) {
	if entry.Attributes == nil {
		entry.Attributes = make(map[string]string)
	}

	for k, v := range nsMeta.Annotations {
		entry.Attributes["k8s-ns-annotation/"+k] = v
	}

	for k, v := range nsMeta.Attributes {
		entry.Attributes["k8s-ns/"+k] = v
	}

	entry.Resource["k8s.namespace.uid"] = nsMeta.UID
	if nsMeta.ClusterName != "" {
		entry.Resource["k8s.cluster.name"] = nsMeta.ClusterName
	}
}

func (k *K8sMetadataDecorator) decorateEntryWithPodMetadata(podMeta MetadataCacheEntry, entry *entry.Entry) {
	if entry.Attributes == nil {
		entry.Attributes = make(map[string]string)
	}

	for k, v := range podMeta.Annotations {
		entry.Attributes["k8s-pod-annotation/"+k] = v
	}

	for k, v := range podMeta.Attributes {
		entry.Attributes["k8s-pod/"+k] = v
	}

	entry.Resource["k8s.pod.uid"] = podMeta.UID
	if podMeta.ClusterName != "" {
		entry.Resource["k8s.cluster.name"] = podMeta.ClusterName
	}

	for key, value := range podMeta.AdditionalResourceValues {
		if value != "" {
			entry.Resource[key] = value
		}
	}
}

func findNameOfKind(ownerRefs []metav1.OwnerReference, kind string) string {
	for _, ref := range ownerRefs {
		if ref.Kind == kind {
			return ref.Name
		}
	}
	return ""
}
