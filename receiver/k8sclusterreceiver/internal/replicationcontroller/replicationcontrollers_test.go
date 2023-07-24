// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicationcontroller

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestReplicationController(t *testing.T) {

	rc := &corev1.ReplicationController{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-replicationcontroller-1",
			Namespace: "test-namespace",
			UID:       "test-replicationcontroller-1-uid",
			Labels: map[string]string{
				"app":     "my-app",
				"version": "v1",
			},
		},
		Spec: corev1.ReplicationControllerSpec{
			Replicas: func() *int32 { i := int32(1); return &i }(),
		},
		Status: corev1.ReplicationControllerStatus{AvailableReplicas: 2},
	}

	m := GetMetrics(receivertest.NewNopCreateSettings(), rc)

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
	),
	)
}
