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

package pod // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"

import (
	"fmt"
	"strings"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/container"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
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

func GetMetrics(pod *corev1.Pod, logger *zap.Logger) []*agentmetricspb.ExportMetricsServiceRequest {
	metrics := []*metricspb.Metric{
		{
			MetricDescriptor: podPhaseMetric,
			Timeseries: []*metricspb.TimeSeries{
				utils.GetInt64TimeSeries(int64(phaseToInt(pod.Status.Phase))),
			},
		},
	}

	podRes := getResource(pod)

	containerResByName := map[string]*agentmetricspb.ExportMetricsServiceRequest{}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ContainerID == "" {
			continue
		}

		contLabels := container.GetAllLabels(cs, podRes.Labels, logger)
		containerResByName[cs.Name] = &agentmetricspb.ExportMetricsServiceRequest{Resource: container.GetResource(contLabels)}

		containerResByName[cs.Name].Metrics = container.GetStatusMetrics(cs)
	}

	for _, c := range pod.Spec.Containers {
		cr := containerResByName[c.Name]

		// This likely will not happen since both pod spec and status return
		// information about the same set of containers. However, if there's
		// a mismatch, skip collecting spec metrics.
		if cr == nil {
			continue
		}

		cr.Metrics = append(cr.Metrics, container.GetSpecMetrics(c)...)
	}

	out := []*agentmetricspb.ExportMetricsServiceRequest{
		{
			Resource: podRes,
			Metrics:  metrics,
		},
	}

	out = append(out, listResourceMetrics(containerResByName)...)

	return out
}

func listResourceMetrics(rms map[string]*agentmetricspb.ExportMetricsServiceRequest) []*agentmetricspb.ExportMetricsServiceRequest {
	out := make([]*agentmetricspb.ExportMetricsServiceRequest, len(rms))

	i := 0
	for _, rm := range rms {
		out[i] = rm
		i++
	}

	return out
}

// getResource returns a proto representation of the pod.
func getResource(pod *corev1.Pod) *resourcepb.Resource {
	return &resourcepb.Resource{
		Type: constants.K8sType,
		Labels: map[string]string{
			conventions.AttributeK8SPodUID:        string(pod.UID),
			conventions.AttributeK8SPodName:       pod.Name,
			conventions.AttributeK8SNodeName:      pod.Spec.NodeName,
			conventions.AttributeK8SNamespaceName: pod.Namespace,
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

// GetMetadata returns all metadata associated with the pod.
func GetMetadata(pod *corev1.Pod, mc *metadata.Store, logger *zap.Logger) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	meta := maps.MergeStringMaps(map[string]string{}, pod.Labels)

	meta[podCreationTime] = pod.CreationTimestamp.Format(time.RFC3339)

	for _, or := range pod.OwnerReferences {
		kind := strings.ToLower(or.Kind)
		meta[metadata.GetOTelNameFromKind(kind)] = or.Name
		meta[metadata.GetOTelUIDFromKind(kind)] = string(or.UID)

		// defer syncing replicaset and job workload metadata.
		if or.Kind == constants.K8sKindReplicaSet || or.Kind == constants.K8sKindJob {
			continue
		}
		meta[constants.K8sKeyWorkLoadKind] = or.Kind
		meta[constants.K8sKeyWorkLoadName] = or.Name
	}

	if mc.Services != nil {
		meta = maps.MergeStringMaps(meta, getPodServiceTags(pod, mc.Services))
	}

	if mc.Jobs != nil {
		meta = maps.MergeStringMaps(meta, collectPodJobProperties(pod, mc.Jobs, logger))
	}

	if mc.ReplicaSets != nil {
		meta = maps.MergeStringMaps(meta, collectPodReplicaSetProperties(pod, mc.ReplicaSets, logger))
	}

	podID := experimentalmetricmetadata.ResourceID(pod.UID)
	return metadata.MergeKubernetesMetadataMaps(map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		podID: {
			ResourceIDKey: conventions.AttributeK8SPodUID,
			ResourceID:    podID,
			Metadata:      meta,
		},
	}, getPodContainerProperties(pod))
}

// collectPodJobProperties checks if pod owner of type Job is cached. Check owners reference
// on Job to see if it was created by a CronJob. Sync metadata accordingly.
func collectPodJobProperties(pod *corev1.Pod, jobStore cache.Store, logger *zap.Logger) map[string]string {
	jobRef := utils.FindOwnerWithKind(pod.OwnerReferences, constants.K8sKindJob)
	if jobRef != nil {
		job, exists, err := jobStore.GetByKey(utils.GetIDForCache(pod.Namespace, jobRef.Name))
		if err != nil {
			logError(err, jobRef, pod.UID, logger)
			return nil
		} else if !exists {
			logWarning(jobRef, pod.UID, logger)
			return nil
		}

		jobObj := job.(*batchv1.Job)
		if cronJobRef := utils.FindOwnerWithKind(jobObj.OwnerReferences, constants.K8sKindCronJob); cronJobRef != nil {
			return getWorkloadProperties(cronJobRef, conventions.AttributeK8SCronJobName)
		}
		return getWorkloadProperties(jobRef, conventions.AttributeK8SJobName)
	}
	return nil
}

// collectPodReplicaSetProperties checks if pod owner of type ReplicaSet is cached. Check owners reference
// on ReplicaSet to see if it was created by a Deployment. Sync metadata accordingly.
func collectPodReplicaSetProperties(pod *corev1.Pod, replicaSetstore cache.Store, logger *zap.Logger) map[string]string {
	rsRef := utils.FindOwnerWithKind(pod.OwnerReferences, constants.K8sKindReplicaSet)
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
		if deployRef := utils.FindOwnerWithKind(replicaSetObj.OwnerReferences, constants.K8sKindDeployment); deployRef != nil {
			return getWorkloadProperties(deployRef, conventions.AttributeK8SDeploymentName)
		}
		return getWorkloadProperties(rsRef, conventions.AttributeK8SReplicaSetName)
	}
	return nil
}

func logWarning(ref *v1.OwnerReference, podUID types.UID, logger *zap.Logger) {
	logger.Warn(
		"Resource does not exist in store, properties from it will not be synced.",
		zap.String(conventions.AttributeK8SPodUID, string(podUID)),
		zap.String(conventions.AttributeK8SJobUID, string(ref.UID)),
	)
}

func logError(err error, ref *v1.OwnerReference, podUID types.UID, logger *zap.Logger) {
	logger.Error(
		"Failed to get resource from store, properties from it will not be synced.",
		zap.String(conventions.AttributeK8SPodUID, string(podUID)),
		zap.String(conventions.AttributeK8SJobUID, string(ref.UID)),
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
			properties[fmt.Sprintf("%s%s", constants.K8sServicePrefix, serObj.Name)] = ""
		}
	}

	return properties
}

// getWorkloadProperties returns workload metadata for provided owner reference.
func getWorkloadProperties(ref *v1.OwnerReference, labelKey string) map[string]string {
	uidKey := metadata.GetOTelUIDFromKind(strings.ToLower(ref.Kind))
	return map[string]string{
		constants.K8sKeyWorkLoadKind: ref.Kind,
		constants.K8sKeyWorkLoadName: ref.Name,
		labelKey:                     ref.Name,
		uidKey:                       string(ref.UID),
	}
}

func getPodContainerProperties(pod *corev1.Pod) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{}
	for _, cs := range pod.Status.ContainerStatuses {
		// Skip if container id returned is empty.
		if cs.ContainerID == "" {
			continue
		}

		md := container.GetMetadata(cs)
		km[md.ResourceID] = md
	}
	return km
}
