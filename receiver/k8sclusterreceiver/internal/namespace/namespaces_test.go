// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/otel/semconv/v1.18.0"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNamespaceMetrics(t *testing.T) {
	n := testutils.NewNamespace("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopSettings(metadata.Type))
	RecordMetrics(mb, n, ts)
	m := mb.Emit()

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

func TestNamespaceMetadata(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			UID:               types.UID("test-namespace-uid"),
			Name:              "test-namespace",
			Namespace:         "default",
			CreationTimestamp: v1.Time{Time: time.Now()},
		},
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}

	meta := GetMetadata(ns)

	require.NotNil(t, meta)
	require.Contains(t, meta, experimentalmetricmetadata.ResourceID("test-namespace-uid"))
	require.Equal(t, "test-namespace", meta[experimentalmetricmetadata.ResourceID("test-namespace-uid")].Metadata[string(conventions.K8SNamespaceNameKey)])
	require.Equal(t, "active", meta[experimentalmetricmetadata.ResourceID("test-namespace-uid")].Metadata["k8s.namespace.phase"])
	require.Equal(t, ns.CreationTimestamp.Format(time.RFC3339), meta[experimentalmetricmetadata.ResourceID("test-namespace-uid")].Metadata["k8s.namespace.creation_timestamp"])
}
