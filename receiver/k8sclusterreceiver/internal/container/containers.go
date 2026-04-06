// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/container"

import (
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// Keys for container metadata used for entity attributes.
	containerKeyStatus         = "container.status"
	containerKeyStatusReason   = "container.status.reason"
	containerCreationTimestamp = "container.creation_timestamp"
	containerName              = "k8s.container.name"
	containerImageName         = "container.image.name"
	containerImageTag          = "container.image.tag"

	// Values for container metadata
	containerStatusRunning    = "running"
	containerStatusWaiting    = "waiting"
	containerStatusTerminated = "terminated"
)

var allContainerStatusReasons = []metadata.AttributeK8sContainerStatusReason{
	metadata.AttributeK8sContainerStatusReasonContainerCreating,
	metadata.AttributeK8sContainerStatusReasonCrashLoopBackOff,
	metadata.AttributeK8sContainerStatusReasonCreateContainerConfigError,
	metadata.AttributeK8sContainerStatusReasonErrImagePull,
	metadata.AttributeK8sContainerStatusReasonImagePullBackOff,
	metadata.AttributeK8sContainerStatusReasonOOMKilled,
	metadata.AttributeK8sContainerStatusReasonCompleted,
	metadata.AttributeK8sContainerStatusReasonError,
	metadata.AttributeK8sContainerStatusReasonContainerCannotRun,
}

// RecordSpecMetrics metricizes values from the container spec.
// This includes values like resource requests and limits.
func RecordSpecMetrics(logger *zap.Logger, mb *metadata.MetricsBuilder, c corev1.Container, pod *corev1.Pod, ts pcommon.Timestamp) {
	var cs *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == c.Name {
			cs = &pod.Status.ContainerStatuses[i]
			break
		}
	}

	var containerID string
	if cs != nil {
		containerID = cs.ContainerID
	}
	e := metadata.NewK8sContainerEntity(stripContainerID(containerID))
	e.SetK8sContainerName(c.Name)
	if cs != nil && cs.LastTerminationState.Terminated != nil {
		e.SetK8sContainerStatusLastTerminatedReason(cs.LastTerminationState.Terminated.Reason)
	}
	image, err := docker.ParseImageName(cs.Image)
	if err != nil {
		docker.LogParseError(err, cs.Image, logger)
	} else {
		e.SetContainerImageName(image.Repository)
		e.SetContainerImageTag(image.Tag)
	}

	e.SetK8sPodName(pod.Name)
	e.SetK8sPodUID(string(pod.UID))
	e.SetK8sNamespaceName(pod.Namespace)
	e.SetK8sNodeName(pod.Spec.NodeName)

	eb := mb.ForK8sContainer(e)

	for k, r := range c.Resources.Requests {
		//exhaustive:ignore
		switch k {
		case corev1.ResourceCPU:
			eb.RecordK8sContainerCPURequestDataPoint(ts, float64(r.MilliValue())/1000.0)
		case corev1.ResourceMemory:
			eb.RecordK8sContainerMemoryRequestDataPoint(ts, r.Value())
		case corev1.ResourceStorage:
			eb.RecordK8sContainerStorageRequestDataPoint(ts, r.Value())
		case corev1.ResourceEphemeralStorage:
			eb.RecordK8sContainerEphemeralstorageRequestDataPoint(ts, r.Value())
		default:
			logger.Debug("unsupported request type", zap.Any("type", k))
		}
	}
	for k, l := range c.Resources.Limits {
		//exhaustive:ignore
		switch k {
		case corev1.ResourceCPU:
			eb.RecordK8sContainerCPULimitDataPoint(ts, float64(l.MilliValue())/1000.0)
		case corev1.ResourceMemory:
			eb.RecordK8sContainerMemoryLimitDataPoint(ts, l.Value())
		case corev1.ResourceStorage:
			eb.RecordK8sContainerStorageLimitDataPoint(ts, l.Value())
		case corev1.ResourceEphemeralStorage:
			eb.RecordK8sContainerEphemeralstorageLimitDataPoint(ts, l.Value())
		default:
			logger.Debug("unsupported request type", zap.Any("type", k))
		}
	}

	if cs != nil {
		eb.RecordK8sContainerRestartsDataPoint(ts, int64(cs.RestartCount))
		eb.RecordK8sContainerReadyDataPoint(ts, boolToInt64(cs.Ready))
		switch {
		case cs.State.Running != nil:
			eb.RecordK8sContainerStatusStateDataPoint(ts, 1, metadata.AttributeK8sContainerStatusStateRunning)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateWaiting)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateTerminated)
		case cs.State.Terminated != nil:
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateRunning)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateWaiting)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 1, metadata.AttributeK8sContainerStatusStateTerminated)
		case cs.State.Waiting != nil:
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateRunning)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 1, metadata.AttributeK8sContainerStatusStateWaiting)
			eb.RecordK8sContainerStatusStateDataPoint(ts, 0, metadata.AttributeK8sContainerStatusStateTerminated)
		}

		// Record k8s.container.status.reason metric: for each known reason emit 1 for the current one, 0 otherwise.
		var reason string
		switch {
		case cs.State.Terminated != nil:
			reason = cs.State.Terminated.Reason
		case cs.State.Waiting != nil:
			reason = cs.State.Waiting.Reason
		default:
			reason = ""
		}
		// Emit in deterministic order for test stability.
		for _, attrVal := range allContainerStatusReasons {
			val := int64(0)
			if reason != "" && reason == attrVal.String() {
				val = 1
			}
			eb.RecordK8sContainerStatusReasonDataPoint(ts, val, attrVal)
		}
	}

	eb.Emit()
}

func GetMetadata(pod *corev1.Pod, cs corev1.ContainerStatus, logger *zap.Logger) *metadata.KubernetesMetadata {
	mdata := map[string]string{}

	imageStr := cs.Image
	image, err := docker.ParseImageName(cs.Image)
	if err != nil {
		docker.LogParseError(err, imageStr, logger)
	} else {
		mdata[containerImageName] = image.Repository
		mdata[containerImageTag] = image.Tag
	}
	mdata[containerName] = cs.Name
	mdata[string(conventions.K8SPodNameKey)] = pod.Name
	mdata[string(conventions.K8SPodUIDKey)] = string(pod.UID)
	mdata[string(conventions.K8SNamespaceNameKey)] = pod.Namespace
	mdata[string(conventions.K8SNodeNameKey)] = pod.Spec.NodeName

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
		ResourceIDKey: string(conventions.ContainerIDKey),
		ResourceID:    metadataPkg.ResourceID(stripContainerID(cs.ContainerID)),
		Metadata:      mdata,
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

var re = regexp.MustCompile(`^[\w_-]+://`)

// stripContainerID returns a pure container id without the runtime scheme://.
func stripContainerID(id string) string {
	return re.ReplaceAllString(id, "")
}
