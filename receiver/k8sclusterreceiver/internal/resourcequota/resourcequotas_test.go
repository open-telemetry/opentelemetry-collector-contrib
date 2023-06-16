// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRequestQuotaMetrics(t *testing.T) {
	rq := newResourceQuota("1")
	m := GetMetrics(receivertest.NewNopCreateSettings(), rq)

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

func newResourceQuota(id string) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-resourcequota-" + id,
			UID:       types.UID("test-resourcequota-" + id + "-uid"),
			Namespace: "test-namespace",
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{
				"requests.cpu": *resource.NewQuantity(2, resource.DecimalSI),
			},
			Used: corev1.ResourceList{
				"requests.cpu": *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
	}
}
