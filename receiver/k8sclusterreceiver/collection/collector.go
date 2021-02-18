// Copyright 2020 OpenTelemetry Authors
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

package collection

import (
	"reflect"
	"time"

	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

// TODO: Consider moving some of these constants to
// https://go.opentelemetry.io/collector/blob/main/translator/conventions/opentelemetry.go.

// Resource label keys.
const (
	// TODO: Remove after switch to new Metrics definition
	// Resource Type
	k8sType       = "k8s"
	containerType = "container"

	// Resource labels keys for UID.
	k8sKeyNodeUID                  = "k8s.node.uid"
	k8sKeyNamespaceUID             = "k8s.namespace.uid"
	k8sKeyReplicationControllerUID = "k8s.replicationcontroller.uid"
	k8sKeyHPAUID                   = "k8s.hpa.uid"
	k8sKeyResourceQuotaUID         = "k8s.resourcequota.uid"

	// Resource labels keys for Name.
	k8sKeyNodeName                  = "k8s.node.name"
	k8sKeyReplicationControllerName = "k8s.replicationcontroller.name"
	k8sKeyHPAName                   = "k8s.hpa.name"
	k8sKeyResourceQuotaName         = "k8s.resourcequota.name"

	// Kubernetes resource kinds
	k8sKindCronJob               = "CronJob"
	k8sKindDaemonSet             = "DaemonSet"
	k8sKindDeployment            = "Deployment"
	k8sKindJob                   = "Job"
	k8sKindReplicationController = "ReplicationController"
	k8sKindReplicaSet            = "ReplicaSet"
	k8sStatefulSet               = "StatefulSet"
)

// DataCollector wraps around a metricsStore and a metadaStore exposing
// methods to perform on the underlying stores. DataCollector also provides
// an interface to interact with refactored code from SignalFx Agent which is
// confined to the collection package.
type DataCollector struct {
	logger                 *zap.Logger
	metricsStore           *metricsStore
	metadataStore          *metadataStore
	nodeConditionsToReport []string
}

// newDataCollector returns a DataCollector.
func NewDataCollector(logger *zap.Logger, nodeConditionsToReport []string) *DataCollector {
	return &DataCollector{
		logger: logger,
		metricsStore: &metricsStore{
			metricsCache: map[types.UID][]consumerdata.MetricsData{},
		},
		metadataStore:          &metadataStore{},
		nodeConditionsToReport: nodeConditionsToReport,
	}
}

// SetupMetadataStore initializes a metadata store for the kubernetes object.
func (dc *DataCollector) SetupMetadataStore(o runtime.Object, store cache.Store) {
	dc.metadataStore.setupStore(o, store)
}

func (dc *DataCollector) RemoveFromMetricsStore(obj interface{}) {
	if err := dc.metricsStore.remove(obj.(runtime.Object)); err != nil {
		dc.logger.Error(
			"failed to remove from metric cache",
			zap.String("obj", reflect.TypeOf(obj).String()),
			zap.Error(err),
		)
	}
}

func (dc *DataCollector) UpdateMetricsStore(obj interface{}, rm []*resourceMetrics) {
	if err := dc.metricsStore.update(obj.(runtime.Object), rm); err != nil {
		dc.logger.Error(
			"failed to update metric cache",
			zap.String("obj", reflect.TypeOf(obj).String()),
			zap.Error(err),
		)
	}
}

func (dc *DataCollector) CollectMetricData(currentTime time.Time) []consumerdata.MetricsData {
	return dc.metricsStore.getMetricData(currentTime)
}

// SyncMetrics updates the metric store with latest metrics from the kubernetes object.
func (dc *DataCollector) SyncMetrics(obj interface{}) {
	var rm []*resourceMetrics

	switch o := obj.(type) {
	case *corev1.Pod:
		rm = getMetricsForPod(o)
	case *corev1.Node:
		rm = getMetricsForNode(o, dc.nodeConditionsToReport)
	case *corev1.Namespace:
		rm = getMetricsForNamespace(o)
	case *corev1.ReplicationController:
		rm = getMetricsForReplicationController(o)
	case *corev1.ResourceQuota:
		rm = getMetricsForResourceQuota(o)
	case *appsv1.Deployment:
		rm = getMetricsForDeployment(o)
	case *appsv1.ReplicaSet:
		rm = getMetricsForReplicaSet(o)
	case *appsv1.DaemonSet:
		rm = getMetricsForDaemonSet(o)
	case *appsv1.StatefulSet:
		rm = getMetricsForStatefulSet(o)
	case *batchv1.Job:
		rm = getMetricsForJob(o)
	case *batchv1beta1.CronJob:
		rm = getMetricsForCronJob(o)
	case *v2beta1.HorizontalPodAutoscaler:
		rm = getMetricsForHPA(o)
	default:
		return
	}

	if len(rm) == 0 {
		return
	}

	dc.UpdateMetricsStore(obj, rm)
}

// SyncMetadata updates the metric store with latest metrics from the kubernetes object
func (dc *DataCollector) SyncMetadata(obj interface{}) map[metadata.ResourceID]*KubernetesMetadata {
	km := map[metadata.ResourceID]*KubernetesMetadata{}
	switch o := obj.(type) {
	case *corev1.Pod:
		km = getMetadataForPod(o, dc.metadataStore, dc.logger)
	case *corev1.Node:
		km = getMetadataForNode(o)
	case *corev1.ReplicationController:
		km = getMetadataForReplicationController(o)
	case *appsv1.Deployment:
		km = getMetadataForDeployment(o)
	case *appsv1.ReplicaSet:
		km = getMetadataForReplicaSet(o)
	case *appsv1.DaemonSet:
		km = getMetadataForDaemonSet(o)
	case *appsv1.StatefulSet:
		km = getMetadataForStatefulSet(o)
	case *batchv1.Job:
		km = getMetadataForJob(o)
	case *batchv1beta1.CronJob:
		km = getMetadataForCronJob(o)
	case *v2beta1.HorizontalPodAutoscaler:
		km = getMetadataForHPA(o)
	}

	return km
}
