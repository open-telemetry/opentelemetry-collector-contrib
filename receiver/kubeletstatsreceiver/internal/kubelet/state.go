// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

var allowedWaitingStateReasons = map[string]struct{}{
	"ErrImagePull":         {},
	"ImagePullBackOff":     {},
	"CrashLoopBackOff":     {},
	"ContainerCreating":    {},
	"CreateContainerError": {},
	"InvalidImageName":     {},
}

var allowedTerminatedStateReasons = map[string]struct{}{
	"OOMKilled":          {},
	"Error":              {},
	"ContainerCannotRun": {},
}

func addContainerStateMetrics(mb *metadata.MetricsBuilder, containerStatus *corev1.ContainerStatus, currentTime pcommon.Timestamp) {
	state := containerStatus.State
	switch {
	case state.Running != nil:
		mb.RecordK8sContainerStateDataPoint(currentTime, 1, metadata.AttributeStateRunning, "")
	case state.Waiting != nil:
		if _, found := allowedWaitingStateReasons[state.Waiting.Reason]; found {
			mb.RecordK8sContainerStateDataPoint(currentTime, 1, metadata.AttributeStateWaiting, state.Waiting.Reason)
		}
	case state.Terminated != nil:
		if _, found := allowedTerminatedStateReasons[state.Terminated.Reason]; found {
			mb.RecordK8sContainerStateDataPoint(currentTime, 1, metadata.AttributeStateTerminated, state.Terminated.Reason)
		}
	}

	lastTerminationState := containerStatus.LastTerminationState
	if lastTerminationState.Terminated != nil {
		if _, found := allowedTerminatedStateReasons[lastTerminationState.Terminated.Reason]; found {
			mb.RecordK8sContainerLastTerminationStateDataPoint(currentTime, 1, lastTerminationState.Terminated.Reason)
		}
	}
}

func addPodStateMetrics(mb *metadata.MetricsBuilder, podStatus *corev1.PodStatus, currentTime pcommon.Timestamp) {
	mb.RecordK8sPodStateDataPoint(currentTime, 1, string(podStatus.Phase), podStatus.Reason)
}
