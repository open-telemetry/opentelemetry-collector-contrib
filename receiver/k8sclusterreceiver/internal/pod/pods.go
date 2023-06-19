// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pod // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
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
	imetadataphase "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

const (
	// Keys for pod metadata.
	podCreationTime = "pod.creation_timestamp"
)

// Transform transforms the pod to remove the fields that we don't use to reduce RAM utilization.
// IMPORTANT: Make sure to update this function when using a new pod fields.
func Transform(pod *corev1.Pod) *corev1.Pod {
	newPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID:       pod.ObjectMeta.UID,
			Name:      pod.ObjectMeta.Name,
			Namespace: pod.ObjectMeta.Namespace,
			Labels:    pod.ObjectMeta.Labels,
		},
		Spec: corev1.PodSpec{
			NodeName: pod.Spec.NodeName,
		},
		Status: corev1.PodStatus{
			Phase: pod.Status.Phase,
		},
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ContainerID == "" {
			continue
		}
		newPod.Status.ContainerStatuses = append(newPod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         cs.Name,
			Image:        cs.Image,
			ContainerID:  cs.ContainerID,
			RestartCount: cs.RestartCount,
			Ready:        cs.Ready,
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

func GetMetrics(set receiver.CreateSettings, pod *corev1.Pod) pmetric.Metrics {
	mbphase := imetadataphase.NewMetricsBuilder(imetadataphase.DefaultMetricsBuilderConfig(), set)
	ts := pcommon.NewTimestampFromTime(time.Now())
	mbphase.RecordK8sPodPhaseDataPoint(ts, int64(phaseToInt(pod.Status.Phase)))
	metrics := mbphase.Emit(imetadataphase.WithK8sNamespaceName(pod.Namespace), imetadataphase.WithK8sNodeName(pod.Spec.NodeName), imetadataphase.WithK8sPodName(pod.Name), imetadataphase.WithK8sPodUID(string(pod.UID)), imetadataphase.WithOpencensusResourcetype("k8s"))

	for _, c := range pod.Spec.Containers {
		specMetrics := container.GetSpecMetrics(set, c, pod)
		specMetrics.ResourceMetrics().MoveAndAppendTo(metrics.ResourceMetrics())
	}

	return metrics
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
		md := container.GetMetadata(cs)
		km[md.ResourceID] = md
	}
	return km
}
