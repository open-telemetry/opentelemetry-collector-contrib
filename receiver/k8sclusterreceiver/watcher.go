// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	quotainformersv1 "github.com/openshift/client-go/quota/informers/externalversions"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/demonset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicaset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"
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
	metadataStore       *metadata.Store
	logger              *zap.Logger
	metadataConsumers   []metadataConsumer
	initialTimeout      time.Duration
	initialSyncDone     *atomic.Bool
	initialSyncTimedOut *atomic.Bool
	config              *Config
	entityLogConsumer   consumer.Logs

	// For mocking.
	makeClient               func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error)
	makeOpenShiftQuotaClient func(apiConf k8sconfig.APIConfig) (quotaclientset.Interface, error)
}

type metadataConsumer func(metadata []*experimentalmetricmetadata.MetadataUpdate) error

// newResourceWatcher creates a Kubernetes resource watcher.
func newResourceWatcher(set receiver.Settings, cfg *Config, metadataStore *metadata.Store) *resourceWatcher {
	return &resourceWatcher{
		logger:                   set.Logger,
		metadataStore:            metadataStore,
		initialSyncDone:          &atomic.Bool{},
		initialSyncTimedOut:      &atomic.Bool{},
		initialTimeout:           defaultInitialSyncTimeout,
		config:                   cfg,
		makeClient:               k8sconfig.MakeClient,
		makeOpenShiftQuotaClient: k8sconfig.MakeOpenShiftQuotaClient,
	}
}

func (rw *resourceWatcher) initialize() error {
	client, err := rw.makeClient(rw.config.APIConfig)
	if err != nil {
		return fmt.Errorf("Failed to create Kubernetes client: %w", err)
	}
	rw.client = client

	if rw.config.Distribution == distributionOpenShift {
		rw.osQuotaClient, err = rw.makeOpenShiftQuotaClient(rw.config.APIConfig)
		if err != nil {
			return fmt.Errorf("Failed to create OpenShift quota API client: %w", err)
		}
	}

	err = rw.prepareSharedInformerFactory()
	if err != nil {
		return err
	}

	return nil
}

func (rw *resourceWatcher) prepareSharedInformerFactory() error {
	factory := informers.NewSharedInformerFactoryWithOptions(rw.client, rw.config.MetadataCollectionInterval)

	// Map of supported group version kinds by name of a kind.
	// If none of the group versions are supported by k8s server for a specific kind,
	// informer for that kind won't be set and a warning message is thrown.
	// This map should be kept in sync with what can be provided by the supported k8s server versions.
	supportedKinds := map[string][]schema.GroupVersionKind{
		"Pod":                     {gvk.Pod},
		"Node":                    {gvk.Node},
		"Namespace":               {gvk.Namespace},
		"ReplicationController":   {gvk.ReplicationController},
		"ResourceQuota":           {gvk.ResourceQuota},
		"Service":                 {gvk.Service},
		"DaemonSet":               {gvk.DaemonSet},
		"Deployment":              {gvk.Deployment},
		"ReplicaSet":              {gvk.ReplicaSet},
		"StatefulSet":             {gvk.StatefulSet},
		"Job":                     {gvk.Job},
		"CronJob":                 {gvk.CronJob},
		"HorizontalPodAutoscaler": {gvk.HorizontalPodAutoscaler},
	}

	for kind, gvks := range supportedKinds {
		anySupported := false
		for _, gvk := range gvks {
			supported, err := rw.isKindSupported(gvk)
			if err != nil {
				return err
			}
			if supported {
				anySupported = true
				rw.setupInformerForKind(gvk, factory)
			}
		}
		if !anySupported {
			rw.logger.Warn("Server doesn't support any of the group versions defined for the kind",
				zap.String("kind", kind))
		}
	}

	if rw.osQuotaClient != nil {
		quotaFactory := quotainformersv1.NewSharedInformerFactory(rw.osQuotaClient, 0)
		rw.setupInformer(gvk.ClusterResourceQuota, quotaFactory.Quota().V1().ClusterResourceQuotas().Informer())
		rw.informerFactories = append(rw.informerFactories, quotaFactory)
	}
	rw.informerFactories = append(rw.informerFactories, factory)

	return nil
}

func (rw *resourceWatcher) isKindSupported(gvk schema.GroupVersionKind) (bool, error) {
	resources, err := rw.client.Discovery().ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		if apierrors.IsNotFound(err) { // if the discovery endpoint isn't present, assume group version is not supported
			rw.logger.Debug("Group version is not supported", zap.String("group", gvk.GroupVersion().String()))
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch group version details: %w", err)
	}

	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			return true, nil
		}
	}
	return false, nil
}

func (rw *resourceWatcher) setupInformerForKind(kind schema.GroupVersionKind, factory informers.SharedInformerFactory) {
	switch kind {
	case gvk.Pod:
		rw.setupInformer(kind, factory.Core().V1().Pods().Informer())
	case gvk.Node:
		rw.setupInformer(kind, factory.Core().V1().Nodes().Informer())
	case gvk.Namespace:
		rw.setupInformer(kind, factory.Core().V1().Namespaces().Informer())
	case gvk.ReplicationController:
		rw.setupInformer(kind, factory.Core().V1().ReplicationControllers().Informer())
	case gvk.ResourceQuota:
		rw.setupInformer(kind, factory.Core().V1().ResourceQuotas().Informer())
	case gvk.Service:
		rw.setupInformer(kind, factory.Core().V1().Services().Informer())
	case gvk.DaemonSet:
		rw.setupInformer(kind, factory.Apps().V1().DaemonSets().Informer())
	case gvk.Deployment:
		rw.setupInformer(kind, factory.Apps().V1().Deployments().Informer())
	case gvk.ReplicaSet:
		rw.setupInformer(kind, factory.Apps().V1().ReplicaSets().Informer())
	case gvk.StatefulSet:
		rw.setupInformer(kind, factory.Apps().V1().StatefulSets().Informer())
	case gvk.Job:
		rw.setupInformer(kind, factory.Batch().V1().Jobs().Informer())
	case gvk.CronJob:
		rw.setupInformer(kind, factory.Batch().V1().CronJobs().Informer())
	case gvk.HorizontalPodAutoscaler:
		rw.setupInformer(kind, factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer())
	default:
		rw.logger.Error("Could not setup an informer for provided group version kind",
			zap.String("group version kind", kind.String()))
	}
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

// setupInformer adds event handlers to informers and setups a metadataStore.
func (rw *resourceWatcher) setupInformer(gvk schema.GroupVersionKind, informer cache.SharedIndexInformer) {
	err := informer.SetTransform(transformObject)
	if err != nil {
		rw.logger.Error("error setting informer transform function", zap.Error(err))
	}
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rw.onAdd,
		UpdateFunc: rw.onUpdate,
	})
	if err != nil {
		rw.logger.Error("error adding event handler to informer", zap.Error(err))
	}
	rw.metadataStore.Setup(gvk, informer.GetStore())
}

func (rw *resourceWatcher) onAdd(obj any) {
	rw.waitForInitialInformerSync()

	// Sync metadata only if there's at least one destination for it to sent.
	if !rw.hasDestination() {
		return
	}

	rw.syncMetadataUpdate(map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{}, rw.objMetadata(obj))
}

func (rw *resourceWatcher) hasDestination() bool {
	return len(rw.metadataConsumers) != 0 || rw.entityLogConsumer != nil
}

func (rw *resourceWatcher) onUpdate(oldObj, newObj any) {
	rw.waitForInitialInformerSync()

	// Sync metadata only if there's at least one destination for it to sent.
	if !rw.hasDestination() {
		return
	}

	rw.syncMetadataUpdate(rw.objMetadata(oldObj), rw.objMetadata(newObj))
}

// objMetadata returns the metadata for the given object.
func (rw *resourceWatcher) objMetadata(obj any) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	switch o := obj.(type) {
	case *corev1.Pod:
		return pod.GetMetadata(o, rw.metadataStore, rw.logger)
	case *corev1.Node:
		return node.GetMetadata(o)
	case *corev1.ReplicationController:
		return replicationcontroller.GetMetadata(o)
	case *appsv1.Deployment:
		return deployment.GetMetadata(o)
	case *appsv1.ReplicaSet:
		return replicaset.GetMetadata(o)
	case *appsv1.DaemonSet:
		return demonset.GetMetadata(o)
	case *appsv1.StatefulSet:
		return statefulset.GetMetadata(o)
	case *batchv1.Job:
		return jobs.GetMetadata(o)
	case *batchv1.CronJob:
		return cronjob.GetMetadata(o)
	case *autoscalingv2.HorizontalPodAutoscaler:
		return hpa.GetMetadata(o)
	}
	return nil
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
	exporters map[component.ID]component.Component,
	metadataExportersFromConfig []string,
) error {
	var out []metadataConsumer

	metadataExportersSet := utils.StringSliceToMap(metadataExportersFromConfig)
	if err := validateMetadataExporters(metadataExportersSet, exporters); err != nil {
		return fmt.Errorf("failed to configure metadata_exporters: %w", err)
	}

	for cfg, exp := range exporters {
		if !metadataExportersSet[cfg.String()] {
			continue
		}
		kme, ok := exp.(experimentalmetricmetadata.MetadataExporter)
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

func validateMetadataExporters(metadataExporters map[string]bool, exporters map[component.ID]component.Component) error {
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

func (rw *resourceWatcher) syncMetadataUpdate(oldMetadata, newMetadata map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	metadataUpdate := metadata.GetMetadataUpdate(oldMetadata, newMetadata)
	if len(metadataUpdate) != 0 {
		for _, consume := range rw.metadataConsumers {
			_ = consume(metadataUpdate)
		}
	}

	if rw.entityLogConsumer != nil {
		// Represent metadata update as entity events.
		entityEvents := metadata.GetEntityEvents(oldMetadata, newMetadata, timestamp, rw.config.MetadataCollectionInterval)

		// Convert entity events to log representation.
		logs := entityEvents.ConvertAndMoveToLogs()

		if logs.LogRecordCount() != 0 {
			err := rw.entityLogConsumer.ConsumeLogs(context.Background(), logs)
			if err != nil {
				rw.logger.Error("Error sending entity events to the consumer", zap.Error(err))

				// Note: receiver contract says that we need to retry sending if the
				// returned error is not Permanent. However, we are not doing it here.
				// Instead, we rely on the fact the metadata is collected periodically
				// and the entity events will be delivered on the next cycle. This is
				// fine because we deliver cumulative entity state.
				// This allows us to avoid stressing the Collector or its destination
				// unnecessarily (typically non-Permanent errors happen in stressed conditions).
				// The periodic collection will be implemented later, see
				// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24413
			}
		}
	}
}
