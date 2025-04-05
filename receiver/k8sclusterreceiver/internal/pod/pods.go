// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pod // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"

import (
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/container"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/service"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

const (
	// Keys for pod metadata and entity attributes. These are NOT used by resource attributes.
	podCreationTime = "pod.creation_timestamp"
	podPhase        = "k8s.pod.phase"
	podStatusReason = "k8s.pod.status_reason"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function before using new pod fields.
func Transform(pod *corev1.Pod) *corev1.Pod {
	newPod := &corev1.Pod{
		ObjectMeta: metadata.TransformObjectMeta(pod.ObjectMeta),
		Spec: corev1.PodSpec{
			NodeName: pod.Spec.NodeName,
		},
		Status: corev1.PodStatus{
			Phase:    pod.Status.Phase,
			QOSClass: pod.Status.QOSClass,
			Reason:   pod.Status.Reason,
		},
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ContainerID == "" {
			continue
		}
		newPod.Status.ContainerStatuses = append(newPod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:                 cs.Name,
			Image:                cs.Image,
			ContainerID:          cs.ContainerID,
			RestartCount:         cs.RestartCount,
			Ready:                cs.Ready,
			State:                cs.State,
			LastTerminationState: cs.LastTerminationState,
		})
	}
	for _, c := range pod.Spec.Containers {
		newPod.Spec.Containers = append(newPod.Spec.Containers, corev1.Container{
			Name: c.Name,
			Resources: corev1.ResourceRequirements{
				Requests: c.Resources.Requests,
				Limits:   c.Resources.Limits,
			},
		})
	}
	return newPod
}

func RecordMetrics(logger *zap.Logger, mb *metadata.MetricsBuilder, pod *corev1.Pod, ts pcommon.Timestamp) {
	mb.RecordK8sPodPhaseDataPoint(ts, int64(phaseToInt(pod.Status.Phase)))
	mb.RecordK8sPodStatusReasonDataPoint(ts, int64(reasonToInt(pod.Status.Reason)))
	rb := mb.NewResourceBuilder()
	rb.SetK8sNamespaceName(pod.Namespace)
	rb.SetK8sNodeName(pod.Spec.NodeName)
	rb.SetK8sPodName(pod.Name)
	rb.SetK8sPodUID(string(pod.UID))
	rb.SetK8sPodQosClass(string(pod.Status.QOSClass))
	mb.EmitForResource(metadata.WithResource(rb.Emit()))

	for _, c := range pod.Spec.Containers {
		container.RecordSpecMetrics(logger, mb, c, pod, ts)
	}
}

func reasonToInt(reason string) int32 {
	switch reason {
	case "Evicted":
		return 1
	case "NodeAffinity":
		return 2
	case "NodeLost":
		return 3
	case "Shutdown":
		return 4
	case "UnexpectedAdmissionError":
		return 5
	default:
		return 6
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
	phase := pod.Status.Phase
	if phase == "" {
		phase = corev1.PodUnknown
	}
	meta[podPhase] = string(phase)
	reason := pod.Status.Reason
	if reason != "" {
		meta[podStatusReason] = reason
	}

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

	if store := mc.Get(gvk.Service); store != nil {
		meta = maps.MergeStringMaps(meta, service.GetPodServiceTags(pod, store))
	}

	if store := mc.Get(gvk.Job); store != nil {
		meta = maps.MergeStringMaps(meta, collectPodJobProperties(pod, store, logger))
	}

	if store := mc.Get(gvk.ReplicaSet); store != nil {
		meta = maps.MergeStringMaps(meta, collectPodReplicaSetProperties(pod, store, logger))
	}

	meta[constants.K8sKeyNamespaceName] = pod.Namespace
	meta[constants.K8sKeyPodName] = pod.Name

	podID := experimentalmetricmetadata.ResourceID(pod.UID)
	return metadata.MergeKubernetesMetadataMaps(map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{
		podID: {
			EntityType:    "k8s.pod",
			ResourceIDKey: conventions.AttributeK8SPodUID,
			ResourceID:    podID,
			Metadata:      meta,
		},
	}, getPodContainerProperties(pod, logger))
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
			logDebug(jobRef, pod.UID, logger)
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
			logDebug(rsRef, pod.UID, logger)
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

func logDebug(ref *v1.OwnerReference, podUID types.UID, logger *zap.Logger) {
	logger.Debug(
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

func getPodContainerProperties(pod *corev1.Pod, logger *zap.Logger) map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata {
	km := map[experimentalmetricmetadata.ResourceID]*metadata.KubernetesMetadata{}
	for _, cs := range pod.Status.ContainerStatuses {
		md := container.GetMetadata(pod, cs, logger)
		km[md.ResourceID] = md
	}
	return km
}
