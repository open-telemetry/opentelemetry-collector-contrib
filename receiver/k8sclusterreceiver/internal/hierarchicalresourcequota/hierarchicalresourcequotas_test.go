// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hierarchicalresourcequota

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHierarchicalRequestQuotaMetrics(t *testing.T) {
	hrq := testutils.NewHierarchicalResourceQuota("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mbc := metadata.DefaultMetricsBuilderConfig()
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopCreateSettings())
	RecordMetrics(mb, hrq, ts)
	m := mb.Emit()

	expected := pmetric.NewMetrics()
	require.NoError(t, pmetrictest.CompareMetrics(expected, m))
}

func TestHierarchicalRequestQuotaMetricsEnabled(t *testing.T) {
	hrq := testutils.NewHierarchicalResourceQuota("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.ResourceAttributes.K8sHierarchicalresourcequotaName.Enabled = true
	mbc.ResourceAttributes.K8sHierarchicalresourcequotaUID.Enabled = true
	mbc.Metrics.K8sHierarchicalResourceQuotaHardLimit.Enabled = true
	mbc.Metrics.K8sHierarchicalResourceQuotaUsed.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopCreateSettings())
	RecordMetrics(mb, hrq, ts)
	m := mb.Emit()

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}

func TestTransform(t *testing.T) {
	original := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-hierarchicalresourcequota-1",
				"uid":       "test-hierarchicalresourcequota-1-uid",
				"namespace": "default",
			},
			"status": map[string]any{
				"hard": map[string]any{
					"requests.cpu":    "1",
					"requests.memory": "1Gi",
				},
				"used": map[string]any{
					"requests.cpu":    "500m",
					"requests.memory": "512Mi",
				},
			},
		},
	}
	wantHRQ := &v1alpha2.HierarchicalResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hierarchicalresourcequota-1",
			Namespace: "default",
			UID:       "test-hierarchicalresourcequota-1-uid",
		},
		Status: v1alpha2.HierarchicalResourceQuotaStatus{
			Hard: corev1.ResourceList{
				corev1.ResourceName("requests.cpu"):    resource.MustParse("1"),
				corev1.ResourceName("requests.memory"): resource.MustParse("1Gi"),
			},
			Used: corev1.ResourceList{
				corev1.ResourceName("requests.cpu"):    resource.MustParse("500m"),
				corev1.ResourceName("requests.memory"): resource.MustParse("512Mi"),
			},
		},
	}
	actual, err := Transform(original)
	assert.NoError(t, err)
	assert.Equal(t, wantHRQ, actual)
}
