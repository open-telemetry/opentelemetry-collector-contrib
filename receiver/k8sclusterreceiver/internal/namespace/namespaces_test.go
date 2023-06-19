// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestNamespaceMetrics(t *testing.T) {
	n := newNamespace("1")
	m := GetMetrics(receivertest.NewNopCreateSettings(), n)

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

func newNamespace(id string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-namespace-" + id,
			Namespace: "test-namespace",
			UID:       types.UID("test-namespace-" + id + "-uid"),
			Labels: map[string]string{
				"foo":  "bar",
				"foo1": "",
			},
		},
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceTerminating,
		},
	}
}
