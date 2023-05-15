// Copyright  The OpenTelemetry Authors
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

package k8sobjectsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/simplelru"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	podv1         = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	eventv1       = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Event"}
	daemonsetv1   = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}
	deploymentv1  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	replicasetv1  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}
	statefulsetv1 = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
	cronjobv1     = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}
	jobv1         = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
)

var cache, _ = simplelru.NewLRU(1000, func(key interface{}, value interface{}) {
	// Callback for key eviction. Anything to do?
})

type k8sobjectsreceiver struct {
	setting         receiver.CreateSettings
	objects         []*K8sObjectsConfig
	stopperChanList []chan struct{}
	client          dynamic.Interface
	consumer        consumer.Logs
	obsrecv         *obsreport.Receiver
	mu              sync.Mutex
}

func newReceiver(params receiver.CreateSettings, config *Config, consumer consumer.Logs) (receiver.Logs, error) {
	transport := "http"
	client, err := config.getDynamicClient()
	if err != nil {
		return nil, err
	}

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             params.ID,
		Transport:              transport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	return &k8sobjectsreceiver{
		client:   client,
		setting:  params,
		consumer: consumer,
		objects:  config.Objects,
		obsrecv:  obsrecv,
		mu:       sync.Mutex{},
	}, nil
}

func (kr *k8sobjectsreceiver) Start(ctx context.Context, host component.Host) error {
	kr.setting.Logger.Info("Object Receiver started")

	for _, object := range kr.objects {
		kr.start(ctx, object)
	}
	return nil
}

func (kr *k8sobjectsreceiver) Shutdown(context.Context) error {
	kr.setting.Logger.Info("Object Receiver stopped")
	kr.mu.Lock()
	for _, stopperChan := range kr.stopperChanList {
		close(stopperChan)
	}
	kr.mu.Unlock()
	return nil
}

func (kr *k8sobjectsreceiver) start(ctx context.Context, object *K8sObjectsConfig) {
	resource := kr.client.Resource(*object.gvr)
	kr.setting.Logger.Info("Started collecting", zap.Any("gvr", object.gvr), zap.Any("mode", object.Mode), zap.Any("namespaces", object.Namespaces))

	switch object.Mode {
	case PullMode:
		if len(object.Namespaces) == 0 {
			go kr.startPull(ctx, object, resource)
		} else {
			for _, ns := range object.Namespaces {
				go kr.startPull(ctx, object, resource.Namespace(ns))
			}
		}

	case WatchMode:
		if len(object.Namespaces) == 0 {
			go kr.startWatch(ctx, object, resource)
		} else {
			for _, ns := range object.Namespaces {
				go kr.startWatch(ctx, object, resource.Namespace(ns))
			}
		}
	}
}

func (kr *k8sobjectsreceiver) startPull(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) {
	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()
	ticker := NewTicker(config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			objects, err := resource.List(ctx, metav1.ListOptions{
				FieldSelector: config.FieldSelector,
				LabelSelector: config.LabelSelector,
			})
			if err != nil {
				kr.setting.Logger.Error("error in pulling object", zap.String("resource", config.gvr.String()), zap.Error(err))
			} else if len(objects.Items) > 0 {
				logs := pullObjectsToLogData(objects, config)

				{
					// Fence modifications in a nested block for ease of rebasing
					for _, item := range objects.Items {
						object := item
						moreLogs, err1 := kr.ensureReferencesAreKnown(ctx, resource, &object)
						if err1 != nil {
							kr.setting.Logger.Error("Cannot retrieve resource versions and owner references", zap.Any("object", toObjectReference(&object)), zap.Error(err))
						} else if moreLogs != nil && moreLogs.LogRecordCount() > 0 {
							moreLogs.ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
						}
					}
				}

				obsCtx := kr.obsrecv.StartLogsOp(ctx)
				err = kr.consumer.ConsumeLogs(obsCtx, logs)
				kr.obsrecv.EndLogsOp(obsCtx, typeStr, logs.LogRecordCount(), err)
			}
		case <-stopperChan:
			return
		}

	}

}

func (kr *k8sobjectsreceiver) startWatch(ctx context.Context, config *K8sObjectsConfig, resource dynamic.ResourceInterface) {

	stopperChan := make(chan struct{})
	kr.mu.Lock()
	kr.stopperChanList = append(kr.stopperChanList, stopperChan)
	kr.mu.Unlock()

	watch, err := resource.Watch(ctx, metav1.ListOptions{
		FieldSelector: config.FieldSelector,
		LabelSelector: config.LabelSelector,
	})
	if err != nil {
		kr.setting.Logger.Error("error in watching object", zap.String("resource", config.gvr.String()), zap.Error(err))
		return
	}

	res := watch.ResultChan()
	for {
		select {
		case data, ok := <-res:
			if !ok {
				kr.setting.Logger.Warn("Watch channel closed unexpectedly", zap.String("resource", config.gvr.String()))
				return
			}
			logs := watchObjectsToLogData(&data, config)

			{
				// Fence modifications in a nested block for ease of rebasing
				object := (data.Object).(*unstructured.Unstructured)
				moreLogs, err := kr.ensureReferencesAreKnown(ctx, resource, object)
				if err != nil {
					kr.setting.Logger.Error("Cannot retrieve resource versions and owner references", zap.Any("object", toObjectReference(object)), zap.Error(err))
				} else if moreLogs != nil && moreLogs.LogRecordCount() > 0 {
					moreLogs.ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
				}
			}

			obsCtx := kr.obsrecv.StartLogsOp(ctx)
			err := kr.consumer.ConsumeLogs(obsCtx, logs)
			kr.obsrecv.EndLogsOp(obsCtx, typeStr, 1, err)
		case <-stopperChan:
			watch.Stop()
			return
		}
	}

}

// Start ticking immediately.
// Ref: https://stackoverflow.com/questions/32705582/how-to-get-time-tick-to-tick-immediately
func NewTicker(repeat time.Duration) *time.Ticker {
	ticker := time.NewTicker(repeat)
	oc := ticker.C
	nc := make(chan time.Time, 1)
	go func() {
		nc <- time.Now()
		for tm := range oc {
			nc <- tm
		}
	}()
	ticker.C = nc
	return ticker
}

func (kr *k8sobjectsreceiver) ensureReferencesAreKnown(ctx context.Context, resource dynamic.ResourceInterface, unstructured *unstructured.Unstructured) (*plog.Logs, error) {
	gvk := unstructured.GroupVersionKind()

	switch gvk {
	case podv1:
		{
			var pod corev1.Pod
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &pod); err != nil {
				return nil, fmt.Errorf("cannot parse v1.Pod from unstructured: %v", err)
			}

			kr.addCacheEntry(pod.APIVersion, pod.Kind, string(pod.ObjectMeta.UID), pod.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &pod.ObjectMeta.OwnerReferences)
		}
	case eventv1:
		{
			// If the object is an event, we look at its involvedObjects instead
			var event corev1.Event
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &event); err != nil {
				return nil, fmt.Errorf("cannot parse v1.Event from unstructured: %v", err)
			}
			// We do not need to create cache keys for the event itself
			return kr.ensureObjectReferenceIsKnown(ctx, resource, &event.InvolvedObject)
		}
	case daemonsetv1:
		{
			var daemonSet appsv1.DaemonSet
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &daemonSet); err != nil {
				return nil, fmt.Errorf("cannot parse appsv1.DaemonSet from unstructured: %v", err)
			}

			kr.addCacheEntry(daemonSet.APIVersion, daemonSet.Kind, string(daemonSet.ObjectMeta.UID), daemonSet.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &daemonSet.ObjectMeta.OwnerReferences)
		}
	case deploymentv1:
		{
			var deployment appsv1.Deployment
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &deployment); err != nil {
				return nil, fmt.Errorf("cannot parse appsv1.Deployment from unstructured: %v", err)
			}

			kr.addCacheEntry(deployment.APIVersion, deployment.Kind, string(deployment.ObjectMeta.UID), deployment.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &deployment.ObjectMeta.OwnerReferences)
		}
	case replicasetv1:
		{
			var replicaSet appsv1.ReplicaSet
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &replicaSet); err != nil {
				return nil, fmt.Errorf("cannot parse appsv1.ReplicaSet from unstructured: %v", err)
			}

			kr.addCacheEntry(replicaSet.APIVersion, replicaSet.Kind, string(replicaSet.ObjectMeta.UID), replicaSet.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &replicaSet.ObjectMeta.OwnerReferences)
		}
	case statefulsetv1:
		{
			var statefulSet appsv1.StatefulSet
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &statefulSet); err != nil {
				return nil, fmt.Errorf("cannot parse appsv1.StatefulSet from unstructured: %v", err)
			}

			kr.addCacheEntry(statefulSet.APIVersion, statefulSet.Kind, string(statefulSet.ObjectMeta.UID), statefulSet.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &statefulSet.ObjectMeta.OwnerReferences)
		}
	case cronjobv1:
		{
			var cronJob batchv1.CronJob
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &cronJob); err != nil {
				return nil, fmt.Errorf("cannot parse batchv1.CronJob from unstructured: %v", err)
			}

			kr.addCacheEntry(cronJob.APIVersion, cronJob.Kind, string(cronJob.ObjectMeta.UID), cronJob.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &cronJob.ObjectMeta.OwnerReferences)
		}
	case jobv1:
		{
			var job batchv1.Job
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), &job); err != nil {
				return nil, fmt.Errorf("cannot parse batchv1.Job from unstructured: %v", err)
			}

			kr.addCacheEntry(job.APIVersion, job.Kind, string(job.ObjectMeta.UID), job.ObjectMeta.ResourceVersion)

			return kr.ensureOwnerReferencesAreKnown(ctx, resource, &job.ObjectMeta.OwnerReferences)
		}
	default:
		return nil, fmt.Errorf("unexpected GroupVersionKind: %+v", gvk)
	}
}

func (kr *k8sobjectsreceiver) ensureOwnerReferencesAreKnown(ctx context.Context, resource dynamic.ResourceInterface, ownerReferences *[]metav1.OwnerReference) (*plog.Logs, error) {
	res := plog.NewLogs()
	for _, ownerReference := range *ownerReferences {
		objectReference := corev1.ObjectReference{
			Kind:       ownerReference.Kind,
			Name:       ownerReference.Name,
			APIVersion: ownerReference.APIVersion,
			UID:        ownerReference.UID,
		}

		logs, err := kr.ensureObjectReferenceIsKnown(ctx, resource, &objectReference)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve owner reference '%v': %v", objectReference, err)
		}

		if logs != nil {
			logs.CopyTo(res)
		}
	}

	return &res, nil
}

func (kr *k8sobjectsreceiver) ensureObjectReferenceIsKnown(ctx context.Context, resource dynamic.ResourceInterface, objectRef *corev1.ObjectReference) (*plog.Logs, error) {
	if kr.hasCacheEntry(objectRef.APIVersion, objectRef.Kind, string(objectRef.UID), objectRef.ResourceVersion) {
		return nil, nil
	}

	kr.setting.Logger.Debug("Retrieving object reference", zap.Any("objectReference", objectRef))

	listOptions := metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			APIVersion: objectRef.APIVersion,
			Kind:       objectRef.Kind,
		},
	}

	// We might not have a resource version, if this object reference is created from an owner reference
	if objectRef.ResourceVersion != "" {
		listOptions.ResourceVersion = objectRef.ResourceVersion
		listOptions.ResourceVersionMatch = metav1.ResourceVersionMatchExact
	}

	res, err := resource.List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("cannot list %s/%s for resource version '%s': %v", objectRef.APIVersion, objectRef.Kind, objectRef.ResourceVersion, err)
	}

	cfg := K8sObjectsConfig{
		gvr: &schema.GroupVersionResource{
			Version:  objectRef.APIVersion,
			Resource: strings.ToLower(objectRef.Kind),
		},
	}

	logs := unstructuredListToLogData(res, &cfg)

	kr.addCacheEntry(objectRef.APIVersion, objectRef.Kind, string(objectRef.UID), objectRef.ResourceVersion)

	return &logs, nil
}

func (kr *k8sobjectsreceiver) hasCacheEntry(apiVersion, kind, uid, resourceVersion string) bool {
	return cache.Contains(toCacheKey(apiVersion, kind, uid, resourceVersion))
}

func (kr *k8sobjectsreceiver) addCacheEntry(apiVersion, kind, uid, resourceVersion string) {
	cacheKey := toCacheKey(apiVersion, kind, uid, resourceVersion)

	if cache.Contains(cacheKey) {
		return
	}

	cache.Add(cacheKey, true)

	kr.setting.Logger.Debug("Added cache key", zap.Any("cacheKey", cacheKey))

	if resourceVersion != "" {
		// Also add entry independent from resource version for owner references
		objectCacheKey := toCacheKey(apiVersion, kind, uid, "")

		if !cache.Contains(objectCacheKey) {
			cache.Add(objectCacheKey, true)
			kr.setting.Logger.Debug("Added cache key", zap.Any("cacheKey", objectCacheKey))
		}
	}
}

func toCacheKey(apiVersion, kind, uid, resourceVersion string) string {
	return fmt.Sprintf("%s.%s/%s@%s", apiVersion, kind, uid, resourceVersion)
}

func toObjectReference(obj *unstructured.Unstructured) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion:      obj.GetAPIVersion(),
		Kind:            obj.GetKind(),
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		UID:             obj.GetUID(),
		ResourceVersion: obj.GetResourceVersion(),
	}
}
