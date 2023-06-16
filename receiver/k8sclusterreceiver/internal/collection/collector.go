// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/collection"

import (
	"reflect"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/clusterresourcequota"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/cronjob"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/demonset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/hpa"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/namespace"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicaset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicationcontroller"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/resourcequota"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"
)

// TODO: Consider moving some of these constants to
// https://go.opentelemetry.io/collector/blob/main/model/semconv/opentelemetry.go.

// DataCollector wraps around a metricsStore and a metadaStore exposing
// methods to perform on the underlying stores. DataCollector also provides
// an interface to interact with refactored code from SignalFx Agent which is
// confined to the collection package.
type DataCollector struct {
	settings                 receiver.CreateSettings
	metricsStore             *metricsStore
	metadataStore            *metadata.Store
	nodeConditionsToReport   []string
	allocatableTypesToReport []string
}

// NewDataCollector returns a DataCollector.
func NewDataCollector(set receiver.CreateSettings, nodeConditionsToReport, allocatableTypesToReport []string) *DataCollector {
	return &DataCollector{
		settings: set,
		metricsStore: &metricsStore{
			metricsCache: make(map[types.UID]pmetric.Metrics),
		},
		metadataStore:            &metadata.Store{},
		nodeConditionsToReport:   nodeConditionsToReport,
		allocatableTypesToReport: allocatableTypesToReport,
	}
}

// SetupMetadataStore initializes a metadata store for the kubernetes kind.
func (dc *DataCollector) SetupMetadataStore(gvk schema.GroupVersionKind, store cache.Store) {
	dc.metadataStore.Setup(gvk, store)
}

func (dc *DataCollector) RemoveFromMetricsStore(obj interface{}) {
	if err := dc.metricsStore.remove(obj.(runtime.Object)); err != nil {
		dc.settings.TelemetrySettings.Logger.Error(
			"failed to remove from metric cache",
			zap.String("obj", reflect.TypeOf(obj).String()),
			zap.Error(err),
		)
	}
}

func (dc *DataCollector) UpdateMetricsStore(obj interface{}, md pmetric.Metrics) {
	if err := dc.metricsStore.update(obj.(runtime.Object), md); err != nil {
		dc.settings.TelemetrySettings.Logger.Error(
			"failed to update metric cache",
			zap.String("obj", reflect.TypeOf(obj).String()),
			zap.Error(err),
		)
	}
}

func (dc *DataCollector) CollectMetricData(currentTime time.Time) pmetric.Metrics {
	return dc.metricsStore.getMetricData(currentTime)
}

// SyncMetrics updates the metric store with latest metrics from the kubernetes object.
func (dc *DataCollector) SyncMetrics(obj interface{}) {
	var md pmetric.Metrics

	switch o := obj.(type) {
	case *corev1.Pod:
		md = ocsToMetrics(pod.GetMetrics(o, dc.settings.TelemetrySettings.Logger))
	case *corev1.Node:
		md = ocsToMetrics(node.GetMetrics(o, dc.nodeConditionsToReport, dc.allocatableTypesToReport, dc.settings.TelemetrySettings.Logger))
	case *corev1.Namespace:
		md = ocsToMetrics(namespace.GetMetrics(o))
	case *corev1.ReplicationController:
		md = ocsToMetrics(replicationcontroller.GetMetrics(o))
	case *corev1.ResourceQuota:
		md = resourcequota.GetMetrics(dc.settings, o)
	case *appsv1.Deployment:
		md = deployment.GetMetrics(dc.settings, o)
	case *appsv1.ReplicaSet:
		md = ocsToMetrics(replicaset.GetMetrics(o))
	case *appsv1.DaemonSet:
		md = ocsToMetrics(demonset.GetMetrics(o))
	case *appsv1.StatefulSet:
		md = statefulset.GetMetrics(dc.settings, o)
	case *batchv1.Job:
		md = ocsToMetrics(jobs.GetMetrics(o))
	case *batchv1.CronJob:
		md = ocsToMetrics(cronjob.GetMetrics(o))
	case *batchv1beta1.CronJob:
		md = ocsToMetrics(cronjob.GetMetricsBeta(o))
	case *autoscalingv2.HorizontalPodAutoscaler:
		md = hpa.GetMetrics(dc.settings, o)
	case *autoscalingv2beta2.HorizontalPodAutoscaler:
		md = hpa.GetMetricsBeta(dc.settings, o)
	case *quotav1.ClusterResourceQuota:
		md = ocsToMetrics(clusterresourcequota.GetMetrics(o))
	default:
		return
	}

	if md.DataPointCount() == 0 {
		return
	}

	dc.UpdateMetricsStore(obj, md)
}

// SyncMetadata updates the metric store with latest metrics from the kubernetes object
func (dc *DataCollector) SyncMetadata(obj interface{}) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{}
	switch o := obj.(type) {
	case *corev1.Pod:
		km = pod.GetMetadata(o, dc.metadataStore, dc.settings.TelemetrySettings.Logger)
	case *corev1.Node:
		km = node.GetMetadata(o)
	case *corev1.ReplicationController:
		km = replicationcontroller.GetMetadata(o)
	case *appsv1.Deployment:
		km = deployment.GetMetadata(o)
	case *appsv1.ReplicaSet:
		km = replicaset.GetMetadata(o)
	case *appsv1.DaemonSet:
		km = demonset.GetMetadata(o)
	case *appsv1.StatefulSet:
		km = statefulset.GetMetadata(o)
	case *batchv1.Job:
		km = jobs.GetMetadata(o)
	case *batchv1.CronJob:
		km = cronjob.GetMetadata(o)
	case *batchv1beta1.CronJob:
		km = cronjob.GetMetadataBeta(o)
	case *autoscalingv2.HorizontalPodAutoscaler:
		km = hpa.GetMetadata(o)
	case *autoscalingv2beta2.HorizontalPodAutoscaler:
		km = hpa.GetMetadataBeta(o)
	}

	return km
}

func ocsToMetrics(ocs []*agentmetricspb.ExportMetricsServiceRequest) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, ocm := range ocs {
		internaldata.OCToMetrics(ocm.Node, ocm.Resource, ocm.Metrics).ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}
	return md
}
