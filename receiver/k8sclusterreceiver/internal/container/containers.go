// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/container"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	imetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

const (
	// Keys for container metadata used for entity attributes.
	containerKeyStatus         = "container.status"
	containerKeyStatusReason   = "container.status.reason"
	containerCreationTimestamp = "container.creation_timestamp"

	// Values for container metadata
	containerStatusRunning    = "running"
	containerStatusWaiting    = "waiting"
	containerStatusTerminated = "terminated"
)

// RecordSpecMetrics metricizes values from the container spec.
// This includes values like resource requests and limits.
func RecordSpecMetrics(logger *zap.Logger, mb *imetadata.MetricsBuilder, c corev1.Container, pod *corev1.Pod, ts pcommon.Timestamp) {
	for k, r := range c.Resources.Requests {
		//exhaustive:ignore
		switch k {
		case corev1.ResourceCPU:
			mb.RecordK8sContainerCPURequestDataPoint(ts, float64(r.MilliValue())/1000.0)
		case corev1.ResourceMemory:
			mb.RecordK8sContainerMemoryRequestDataPoint(ts, r.Value())
		case corev1.ResourceStorage:
			mb.RecordK8sContainerStorageRequestDataPoint(ts, r.Value())
		case corev1.ResourceEphemeralStorage:
			mb.RecordK8sContainerEphemeralstorageRequestDataPoint(ts, r.Value())
		default:
			logger.Debug("unsupported request type", zap.Any("type", k))
		}
	}
	for k, l := range c.Resources.Limits {
		//exhaustive:ignore
		switch k {
		case corev1.ResourceCPU:
			mb.RecordK8sContainerCPULimitDataPoint(ts, float64(l.MilliValue())/1000.0)
		case corev1.ResourceMemory:
			mb.RecordK8sContainerMemoryLimitDataPoint(ts, l.Value())
		case corev1.ResourceStorage:
			mb.RecordK8sContainerStorageLimitDataPoint(ts, l.Value())
		case corev1.ResourceEphemeralStorage:
			mb.RecordK8sContainerEphemeralstorageLimitDataPoint(ts, l.Value())
		default:
			logger.Debug("unsupported request type", zap.Any("type", k))
		}
	}

	rb := mb.NewResourceBuilder()
	var containerID string
	var imageStr string
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == c.Name {
			containerID = cs.ContainerID
			imageStr = cs.Image
			mb.RecordK8sContainerRestartsDataPoint(ts, int64(cs.RestartCount))
			mb.RecordK8sContainerReadyDataPoint(ts, boolToInt64(cs.Ready))
			if cs.LastTerminationState.Terminated != nil {
				rb.SetK8sContainerStatusLastTerminatedReason(cs.LastTerminationState.Terminated.Reason)
			}
			break
		}
	}

	rb.SetK8sPodUID(string(pod.UID))
	rb.SetK8sPodName(pod.Name)
	rb.SetK8sNodeName(pod.Spec.NodeName)
	rb.SetK8sNamespaceName(pod.Namespace)
	rb.SetContainerID(utils.StripContainerID(containerID))
	rb.SetK8sContainerName(c.Name)
	image, err := docker.ParseImageName(imageStr)
	if err != nil {
		docker.LogParseError(err, imageStr, logger)
	} else {
		rb.SetContainerImageName(image.Repository)
		rb.SetContainerImageTag(image.Tag)
	}
	mb.EmitForResource(imetadata.WithResource(rb.Emit()))
}

func GetMetadata(cs corev1.ContainerStatus) *metadata.KubernetesMetadata {
	mdata := map[string]string{}

	if cs.State.Running != nil {
		mdata[containerKeyStatus] = containerStatusRunning
		if !cs.State.Running.StartedAt.IsZero() {
			mdata[containerCreationTimestamp] = cs.State.Running.StartedAt.Format(time.RFC3339)
		}
	}

	if cs.State.Terminated != nil {
		mdata[containerKeyStatus] = containerStatusTerminated
		mdata[containerKeyStatusReason] = cs.State.Terminated.Reason
		if !cs.State.Terminated.StartedAt.IsZero() {
			mdata[containerCreationTimestamp] = cs.State.Terminated.StartedAt.Format(time.RFC3339)
		}
	}

	if cs.State.Waiting != nil {
		mdata[containerKeyStatus] = containerStatusWaiting
		mdata[containerKeyStatusReason] = cs.State.Waiting.Reason
	}

	return &metadata.KubernetesMetadata{
		EntityType:    "container",
		ResourceIDKey: conventions.AttributeContainerID,
		ResourceID:    metadataPkg.ResourceID(utils.StripContainerID(cs.ContainerID)),
		Metadata:      mdata,
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
