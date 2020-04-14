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
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opencensus.io/resource/resourcekeys"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/utils"
)

const (
	// Keys for pod properties.
	podCreationTime = "pod.creation_timestamp"
)

var podPhaseMetric = &metricspb.MetricDescriptor{
	Name:        "kubernetes/pod/phase",
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

	containerResByName := make(map[string]*resourceMetrics)

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ContainerID == "" {
			continue
		}

		contLabels := getAllContainerLabels(cs.ContainerID, cs.Name, cs.Image, podRes.Labels)
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
		Type: resourcekeys.K8SType,
		Labels: map[string]string{
			k8sKeyPodUID:                     string(pod.UID),
			resourcekeys.K8SKeyPodName:       pod.Name,
			k8sKeyNodeName:                   pod.Spec.NodeName,
			resourcekeys.K8SKeyNamespaceName: pod.Namespace,
			resourcekeys.K8SKeyClusterName:   pod.ClusterName,
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
func getMetadataForPod(pod *corev1.Pod, mc *metadataStore) []*KubernetesMetadata {
	properties := pod.Labels

	properties[podCreationTime] = pod.CreationTimestamp.Format(time.RFC3339)

	for _, or := range pod.OwnerReferences {
		properties[strings.ToLower(or.Kind)] = or.Name
		properties[strings.ToLower(or.Kind)+"_uid"] = string(or.UID)

		// defer syncing replicaset and job workload properties.
		if or.Kind == "ReplicaSet" || or.Kind == "Job" {
			continue
		}
		properties[k8sKeyWorkLoad] = or.Kind
		properties[k8sKeyWorkLoadName] = or.Name
	}

	properties = utils.MergeStringMaps(properties, getPodServiceTags(pod, mc.stores["Service"]))
	properties = utils.MergeStringMaps(properties,
		collectPodJobProperties(pod, mc.stores["Job"]),
		collectPodReplicaSetProperties(pod, mc.stores["ReplicaSet"]),
	)

	return append([]*KubernetesMetadata{
		{
			resourceIDKey: k8sKeyPodUID,
			resourceID:    string(pod.UID),
			properties:    properties,
		},
	}, getPodContainerProperties(pod)...)
}

// collectPodJobProperties checks if pod owner of type Job is cached. Check owners reference
// on Job to see if it was created by a CronJob. Sync properties accordingly.
func collectPodJobProperties(pod *corev1.Pod, store cache.Store) map[string]string {
	properties := make(map[string]string)

	jobRef := utils.FindOwnerWithKind(pod.OwnerReferences, "Job")
	if jobRef != nil {
		job, ok, _ := store.GetByKey(utils.GetIDForCache(pod.Namespace, jobRef.Name))
		if ok {
			jobObj := job.(*batchv1.Job)
			if cronJobRef := utils.FindOwnerWithKind(jobObj.OwnerReferences, "CronJob"); cronJobRef != nil {
				properties = utils.MergeStringMaps(getPodCronJobProperties(cronJobRef), properties)
			} else {
				properties = utils.MergeStringMaps(
					getPodWorkloadProperties(jobObj.Name, "Job"),
					properties,
				)
			}
		}
	}

	return properties
}

// collectPodReplicaSetProperties checks if pod owner of type ReplicaSet is cached. Check owners reference
// on ReplicaSet to see if it was created by a Deployment. Sync properties accordingly.
func collectPodReplicaSetProperties(pod *corev1.Pod, store cache.Store) map[string]string {
	properties := make(map[string]string)

	rsRef := utils.FindOwnerWithKind(pod.OwnerReferences, "ReplicaSet")
	if rsRef != nil {
		replicaSet, ok, _ := store.GetByKey(utils.GetIDForCache(pod.Namespace, rsRef.Name))
		if ok {
			replicaSetObj := replicaSet.(*appsv1.ReplicaSet)
			if deployRef := utils.FindOwnerWithKind(replicaSetObj.OwnerReferences, "Deployment"); deployRef != nil {
				properties = utils.MergeStringMaps(getPodDeploymentProperties(rsRef), properties)

			} else {
				properties = utils.MergeStringMaps(
					getPodWorkloadProperties(replicaSetObj.Name, "ReplicaSet"),
					properties,
				)
			}
		}
	}
	return properties
}

// getPodServiceTags returns a set of services associated with the pod.
func getPodServiceTags(pod *corev1.Pod, store cache.Store) map[string]string {
	properties := make(map[string]string)
	services := store.List()

	for _, ser := range services {
		serObj := ser.(*corev1.Service)
		if serObj.Namespace == pod.Namespace &&
			labels.Set(serObj.Spec.Selector).AsSelectorPreValidated().Matches(labels.Set(pod.Labels)) {
			properties["kubernetes_service_"+serObj.Name] = ""
		}
	}

	return properties
}

// getPodCronJobProperties returns properties of a CronJob associated with the pod.
func getPodCronJobProperties(cronJobRef *v1.OwnerReference) map[string]string {
	k := strings.ToLower(cronJobRef.Kind)
	return map[string]string{
		k8sKeyWorkLoad:     k,
		k8sKeyWorkLoadName: cronJobRef.Name,
		k8sKeyCronJobName:  cronJobRef.Name,
		k + "_uid":         string(cronJobRef.UID),
	}
}

// getPodDeploymentProperties returns properties of a Deployment associated with the pod.
func getPodDeploymentProperties(rsRef *v1.OwnerReference) map[string]string {
	k := strings.ToLower(rsRef.Kind)
	return map[string]string{
		k8sKeyWorkLoad:       k,
		k8sKeyWorkLoadName:   rsRef.Name,
		k8sKeyDeploymentName: rsRef.Name,
		k + "_uid":           string(rsRef.UID),
	}
}

// getPodWorkloadProperties returns properties of a Kubernetes workload associated with the pod.
func getPodWorkloadProperties(workloadName string, workloadType string) map[string]string {
	return map[string]string{
		k8sKeyWorkLoad:     strings.ToLower(workloadType),
		k8sKeyWorkLoadName: workloadName,
	}
}

func getPodContainerProperties(pod *corev1.Pod) []*KubernetesMetadata {
	rm := make([]*KubernetesMetadata, 0)
	for _, cs := range pod.Status.ContainerStatuses {
		// Skip if container id returned is empty.
		if cs.ContainerID == "" {
			continue
		}

		rm = append(rm, getMetadataForContainer(cs))
	}
	return rm
}
