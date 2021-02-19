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

package collection

import (
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/util"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for pod metadata.
	podCreationTime = "pod.creation_timestamp"
)

var podPhaseMetric = &metricspb.MetricDescriptor{
	Name:        "k8s.pod.phase",
	Description: "Current phase of the pod (1 - Pending, 2 - Running, 3 - Succeeded, 4 - Failed, 5 - Unknown)",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
}

func getMetricsForPod(pod *corev1.Pod) []*resourceMetrics {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: podPhaseMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(phaseToInt(pod.Status.Phase))),
			},
		},
	}

	podRes := getResourceForPod(pod)

	containerResByName := map[string]*resourceMetrics{}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ContainerID == "" {
			continue
		}

		contLabels := getAllContainerLabels(cs, podRes.Labels)
		containerResByName[cs.Name] = &resourceMetrics{resource: getResourceForContainer(contLabels)}

		containerResByName[cs.Name].metrics = getStatusMetricsForContainer(cs)
	}

	for _, c := range pod.Spec.Containers {
		cr := containerResByName[c.Name]

		// This likely will not happen since both pod spec and status return
		// information about the same set of containers. However, if there's
		// a mismatch, skip collecting spec metrics.
		if cr == nil {
			continue
		}

		cr.metrics = append(cr.metrics, getSpecMetricsForContainer(c)...)
	}

	out := []*resourceMetrics{
		{
			resource: podRes,
			metrics:  metrics,
		},
	}

	out = append(out, listResourceMetrics(containerResByName)...)

	return out
}

func listResourceMetrics(rms map[string]*resourceMetrics) []*resourceMetrics {
	out := make([]*resourceMetrics, len(rms))

	i := 0
	for _, rm := range rms {
		out[i] = rm
		i++
	}

	return out
}

// getResourceForPod returns a proto representation of the pod.
func getResourceForPod(pod *corev1.Pod) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: k8sType,
		Labels: map[string]string{
			conventions.AttributeK8sPodUID:    string(pod.UID),
			conventions.AttributeK8sPod:       pod.Name,
			k8sKeyNodeName:                    pod.Spec.NodeName,
			conventions.AttributeK8sNamespace: pod.Namespace,
			conventions.AttributeK8sCluster:   pod.ClusterName,
		},
	}
}

func phaseToInt(phase corev1.PodPhase) int32 {
	switch phase {
	case corev1.PodPending:
		return 1
	case corev1.PodRunning:
		return 2
	case corev1.PodSucceeded:
		return 3
	case corev1.PodFailed:
		return 4
	case corev1.PodUnknown:
		return 5
	default:
		return 5
	}
}

// getMetadataForPod returns all metadata associated with the pod.
func getMetadataForPod(pod *corev1.Pod, mc *metadataStore, logger *zap.Logger) map[metadataPkg.ResourceID]*KubernetesMetadata {
	metadata := util.MergeStringMaps(map[string]string{}, pod.Labels)

	metadata[podCreationTime] = pod.CreationTimestamp.Format(time.RFC3339)

	for _, or := range pod.OwnerReferences {
		metadata[strings.ToLower(or.Kind)] = or.Name
		metadata[strings.ToLower(or.Kind)+"_uid"] = string(or.UID)

		// defer syncing replicaset and job workload metadata.
		if or.Kind == k8sKindReplicaSet || or.Kind == k8sKindJob {
			continue
		}
		metadata[k8sKeyWorkLoadKind] = or.Kind
		metadata[k8sKeyWorkLoadName] = or.Name
	}

	if mc.services != nil {
		metadata = util.MergeStringMaps(metadata,
			getPodServiceTags(pod, mc.services),
		)
	}

	if mc.jobs != nil {
		metadata = util.MergeStringMaps(metadata,
			collectPodJobProperties(pod, mc.jobs, logger),
		)
	}

	if mc.replicaSets != nil {
		metadata = util.MergeStringMaps(metadata,
			collectPodReplicaSetProperties(pod, mc.replicaSets, logger),
		)
	}

	podID := metadataPkg.ResourceID(pod.UID)
	return mergeKubernetesMetadataMaps(map[metadataPkg.ResourceID]*KubernetesMetadata{
		podID: {
			resourceIDKey: conventions.AttributeK8sPodUID,
			resourceID:    podID,
			metadata:      metadata,
		},
	}, getPodContainerProperties(pod))
}

// collectPodJobProperties checks if pod owner of type Job is cached. Check owners reference
// on Job to see if it was created by a CronJob. Sync metadata accordingly.
func collectPodJobProperties(pod *corev1.Pod, JobStore cache.Store, logger *zap.Logger) map[string]string {
	jobRef := utils.FindOwnerWithKind(pod.OwnerReferences, k8sKindJob)
	if jobRef != nil {
		job, exists, err := JobStore.GetByKey(utils.GetIDForCache(pod.Namespace, jobRef.Name))
		if err != nil {
			logError(err, jobRef, pod.UID, logger)
			return nil
		} else if !exists {
			logWarning(jobRef, pod.UID, logger)
			return nil
		}

		jobObj := job.(*batchv1.Job)
		if cronJobRef := utils.FindOwnerWithKind(jobObj.OwnerReferences, k8sKindCronJob); cronJobRef != nil {
			return getWorkloadProperties(cronJobRef, conventions.AttributeK8sCronJob)
		}
		return getWorkloadProperties(jobRef, conventions.AttributeK8sJob)
	}
	return nil
}

// collectPodReplicaSetProperties checks if pod owner of type ReplicaSet is cached. Check owners reference
// on ReplicaSet to see if it was created by a Deployment. Sync metadata accordingly.
func collectPodReplicaSetProperties(pod *corev1.Pod, replicaSetstore cache.Store, logger *zap.Logger) map[string]string {
	rsRef := utils.FindOwnerWithKind(pod.OwnerReferences, k8sKindReplicaSet)
	if rsRef != nil {
		replicaSet, exists, err := replicaSetstore.GetByKey(utils.GetIDForCache(pod.Namespace, rsRef.Name))
		if err != nil {
			logError(err, rsRef, pod.UID, logger)
			return nil
		} else if !exists {
			logWarning(rsRef, pod.UID, logger)
			return nil
		}

		replicaSetObj := replicaSet.(*appsv1.ReplicaSet)
		if deployRef := utils.FindOwnerWithKind(replicaSetObj.OwnerReferences, k8sKindDeployment); deployRef != nil {
			return getWorkloadProperties(deployRef, conventions.AttributeK8sDeployment)
		}
		return getWorkloadProperties(rsRef, conventions.AttributeK8sReplicaSet)
	}
	return nil
}

func logWarning(ref *v1.OwnerReference, podUID types.UID, logger *zap.Logger) {
	logger.Warn(
		"Resource does not exist in store, properties from it will not be synced.",
		zap.String(conventions.AttributeK8sPodUID, string(podUID)),
		zap.String(conventions.AttributeK8sJobUID, string(ref.UID)),
	)
}

func logError(err error, ref *v1.OwnerReference, podUID types.UID, logger *zap.Logger) {
	logger.Error(
		"Failed to get resource from store, properties from it will not be synced.",
		zap.String(conventions.AttributeK8sPodUID, string(podUID)),
		zap.String(conventions.AttributeK8sJobUID, string(ref.UID)),
		zap.Error(err),
	)
}

// getPodServiceTags returns a set of services associated with the pod.
func getPodServiceTags(pod *corev1.Pod, services cache.Store) map[string]string {
	properties := map[string]string{}

	for _, ser := range services.List() {
		serObj := ser.(*corev1.Service)
		if serObj.Namespace == pod.Namespace &&
			labels.Set(serObj.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
			properties["kubernetes_service_"+serObj.Name] = ""
		}
	}

	return properties
}

// getWorkloadProperties returns workload metadata for provided owner reference.
func getWorkloadProperties(ref *v1.OwnerReference, labelKey string) map[string]string {
	kind := ref.Kind
	uidKey := strings.ToLower(kind) + "_uid"
	return map[string]string{
		k8sKeyWorkLoadKind: kind,
		k8sKeyWorkLoadName: ref.Name,
		labelKey:           ref.Name,
		uidKey:             string(ref.UID),
	}
}

func getPodContainerProperties(pod *corev1.Pod) map[metadataPkg.ResourceID]*KubernetesMetadata {
	km := map[metadataPkg.ResourceID]*KubernetesMetadata{}
	for _, cs := range pod.Status.ContainerStatuses {
		// Skip if container id returned is empty.
		if cs.ContainerID == "" {
			continue
		}

		md := getMetadataForContainer(cs)
		km[md.resourceID] = md
	}
	return km
}
