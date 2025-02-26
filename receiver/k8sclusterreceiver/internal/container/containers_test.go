// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetMetadata(t *testing.T) {
	refTime := v1.Now()
	tests := []struct {
		name              string
		containerState    corev1.ContainerState
		expectedStatus    string
		expectedReason    string
		expectedStartedAt string
	}{
		{
			name: "Running container",
			containerState: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: refTime,
				},
			},
			expectedStatus:    containerStatusRunning,
			expectedStartedAt: refTime.Format(time.RFC3339),
		},
		{
			name: "Terminated container",
			containerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ContainerID: "container-id",
					Reason:      "Completed",
					StartedAt:   refTime,
					FinishedAt:  refTime,
					ExitCode:    0,
				},
			},
			expectedStatus:    containerStatusTerminated,
			expectedReason:    "Completed",
			expectedStartedAt: refTime.Format(time.RFC3339),
		},
		{
			name: "Waiting container",
			containerState: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason: "CrashLoopBackOff",
				},
			},
			expectedStatus: containerStatusWaiting,
			expectedReason: "CrashLoopBackOff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := corev1.ContainerStatus{
				State: tt.containerState,
			}
			md := GetMetadata(cs)

			require.NotNil(t, md)
			assert.Equal(t, tt.expectedStatus, md.Metadata[containerKeyStatus])
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, md.Metadata[containerKeyStatusReason])
			}
			if tt.containerState.Running != nil || tt.containerState.Terminated != nil {
				assert.Contains(t, md.Metadata, containerCreationTimestamp)
				assert.Equal(t, tt.expectedStartedAt, md.Metadata[containerCreationTimestamp])
			}
		})
	}
}
