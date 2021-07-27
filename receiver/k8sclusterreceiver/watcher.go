// Copyright 2020, OpenTelemetry Authors
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

package k8sclusterreceiver

import (
	"context"
	"fmt"
	"reflect"
	"time"

	quotav1 "github.com/openshift/api/quota/v1"
	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	quotainformersv1 "github.com/openshift/client-go/quota/informers/externalversions"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

type sharedInformer interface {
	Start(<-chan struct{})
	WaitForCacheSync(<-chan struct{}) map[reflect.Type]bool
}

type resourceWatcher struct {
	client              kubernetes.Interface
	osQuotaClient       quotaclientset.Interface
	informerFactories   []sharedInformer
	dataCollector       *collection.DataCollector
	logger              *zap.Logger
	metadataConsumers   []metadataConsumer
	initialTimeout      time.Duration
	initialSyncDone     *atomic.Bool
	initialSyncTimedOut *atomic.Bool
}

type metadataConsumer func(metadata []*metadata.MetadataUpdate) error

// newResourceWatcher creates a Kubernetes resource watcher.
func newResourceWatcher(
	logger *zap.Logger, client kubernetes.Interface, osQuotaClient quotaclientset.Interface,
	nodeConditionTypesToReport []string, initialSyncTimeout time.Duration) *resourceWatcher {
	rw := &resourceWatcher{
		client:              client,
		osQuotaClient:       osQuotaClient,
		informerFactories:   []sharedInformer{},
		logger:              logger,
		dataCollector:       collection.NewDataCollector(logger, nodeConditionTypesToReport),
		initialSyncDone:     atomic.NewBool(false),
		initialSyncTimedOut: atomic.NewBool(false),
		initialTimeout:      initialSyncTimeout,
	}

	rw.prepareSharedInformerFactory()

	return rw
}

func (rw *resourceWatcher) prepareSharedInformerFactory() {
	factory := informers.NewSharedInformerFactoryWithOptions(rw.client, 0)

	// Add shared informers for each resource type that has to be watched.
	rw.setupInformers(&corev1.Pod{}, factory.Core().V1().Pods().Informer())
	rw.setupInformers(&corev1.Node{}, factory.Core().V1().Nodes().Informer())
	rw.setupInformers(&corev1.Namespace{}, factory.Core().V1().Namespaces().Informer())
	rw.setupInformers(&corev1.ReplicationController{},
		factory.Core().V1().ReplicationControllers().Informer(),
	)
	rw.setupInformers(&corev1.ResourceQuota{}, factory.Core().V1().ResourceQuotas().Informer())
	rw.setupInformers(&corev1.Service{}, factory.Core().V1().Services().Informer())
	rw.setupInformers(&appsv1.DaemonSet{}, factory.Apps().V1().DaemonSets().Informer())
	rw.setupInformers(&appsv1.Deployment{}, factory.Apps().V1().Deployments().Informer())
	rw.setupInformers(&appsv1.ReplicaSet{}, factory.Apps().V1().ReplicaSets().Informer())
	rw.setupInformers(&appsv1.StatefulSet{}, factory.Apps().V1().StatefulSets().Informer())
	rw.setupInformers(&batchv1.Job{}, factory.Batch().V1().Jobs().Informer())
	rw.setupInformers(&batchv1beta1.CronJob{}, factory.Batch().V1beta1().CronJobs().Informer())
	rw.setupInformers(&v2beta1.HorizontalPodAutoscaler{},
		factory.Autoscaling().V2beta1().HorizontalPodAutoscalers().Informer(),
	)

	if rw.osQuotaClient != nil {
		quotaFactory := quotainformersv1.NewSharedInformerFactory(rw.osQuotaClient, 0)
		rw.setupInformers(&quotav1.ClusterResourceQuota{}, quotaFactory.Quota().V1().ClusterResourceQuotas().Informer())
		rw.informerFactories = append(rw.informerFactories, quotaFactory)
	}
	rw.informerFactories = append(rw.informerFactories, factory)
}

// startWatchingResources starts up all informers.
func (rw *resourceWatcher) startWatchingResources(ctx context.Context, inf sharedInformer) context.Context {
	var cancel context.CancelFunc
	timedContextForInitialSync, cancel := context.WithTimeout(ctx, rw.initialTimeout)

	// Start off individual informers in the factory.
	inf.Start(ctx.Done())

	// Ensure cache is synced with initial state, once informers are started up.
	// Note that the event handler can start receiving events as soon as the informers
	// are started. So it's required to ensure that the receiver does not start
	// collecting data before the cache sync since all data may not be available.
	// This method will block either till the timeout set on the context, until
	// the initial sync is complete or the parent context is cancelled.
	inf.WaitForCacheSync(timedContextForInitialSync.Done())
	defer cancel()
	return timedContextForInitialSync
}

// setupInformers adds event handlers to informers and setups a metadataStore.
func (rw *resourceWatcher) setupInformers(o runtime.Object, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rw.onAdd,
		UpdateFunc: rw.onUpdate,
		DeleteFunc: rw.onDelete,
	})
	rw.dataCollector.SetupMetadataStore(o, informer.GetStore())
}

func (rw *resourceWatcher) onAdd(obj interface{}) {
	rw.waitForInitialInformerSync()
	rw.dataCollector.SyncMetrics(obj)

	// Sync metadata only if there's at least one destination for it to sent.
	if len(rw.metadataConsumers) == 0 {
		return
	}

	newMetadata := rw.dataCollector.SyncMetadata(obj)
	rw.syncMetadataUpdate(map[metadata.ResourceID]*collection.KubernetesMetadata{}, newMetadata)
}

func (rw *resourceWatcher) onDelete(obj interface{}) {
	rw.waitForInitialInformerSync()
	rw.dataCollector.RemoveFromMetricsStore(obj)
}

func (rw *resourceWatcher) onUpdate(oldObj, newObj interface{}) {
	rw.waitForInitialInformerSync()
	// Sync metrics from the new object
	rw.dataCollector.SyncMetrics(newObj)

	// Sync metadata only if there's at least one destination for it to sent.
	if len(rw.metadataConsumers) == 0 {
		return
	}

	oldMetadata := rw.dataCollector.SyncMetadata(oldObj)
	newMetadata := rw.dataCollector.SyncMetadata(newObj)

	rw.syncMetadataUpdate(oldMetadata, newMetadata)
}

func (rw *resourceWatcher) waitForInitialInformerSync() {
	if rw.initialSyncDone.Load() || rw.initialSyncTimedOut.Load() {
		return
	}

	// Wait till initial sync is complete or timeout.
	for !rw.initialSyncDone.Load() {
		if rw.initialSyncTimedOut.Load() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (rw *resourceWatcher) setupMetadataExporters(
	exporters map[config.ComponentID]component.Exporter,
	metadataExportersFromConfig []string,
) error {

	var out []metadataConsumer

	metadataExportersSet := utils.StringSliceToMap(metadataExportersFromConfig)
	if err := validateMetadataExporters(metadataExportersSet, exporters); err != nil {
		return fmt.Errorf("failed to configure metadata_exporters: %v", err)
	}

	for cfg, exp := range exporters {
		if !metadataExportersSet[cfg.String()] {
			continue
		}
		kme, ok := exp.(metadata.MetadataExporter)
		if !ok {
			return fmt.Errorf("%s exporter does not implement MetadataExporter", cfg.Name())
		}
		out = append(out, kme.ConsumeMetadata)
		rw.logger.Info("Configured Kubernetes MetadataExporter",
			zap.String("exporter_name", cfg.String()),
		)
	}

	rw.metadataConsumers = out
	return nil
}

func validateMetadataExporters(metadataExporters map[string]bool,
	exporters map[config.ComponentID]component.Exporter) error {

	configuredExporters := map[string]bool{}
	for cfg := range exporters {
		configuredExporters[cfg.String()] = true
	}

	for e := range metadataExporters {
		if !configuredExporters[e] {
			return fmt.Errorf("%s exporter is not in collector config", e)
		}
	}

	return nil
}

func (rw *resourceWatcher) syncMetadataUpdate(oldMetadata,
	newMetadata map[metadata.ResourceID]*collection.KubernetesMetadata) {

	metadataUpdate := collection.GetMetadataUpdate(oldMetadata, newMetadata)
	if len(metadataUpdate) == 0 {
		return
	}

	for _, consume := range rw.metadataConsumers {
		consume(metadataUpdate)
	}
}
