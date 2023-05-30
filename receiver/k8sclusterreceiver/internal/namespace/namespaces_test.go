// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestNamespaceMetrics(t *testing.T) {
	n := newNamespace("1")

	actualResourceMetrics := GetMetrics(n)

	require.Equal(t, 1, len(actualResourceMetrics))

	require.Equal(t, 1, len(actualResourceMetrics[0].Metrics))
	testutils.AssertResource(t, actualResourceMetrics[0].Resource, constants.K8sType,
		map[string]string{
			"k8s.namespace.uid":  "test-namespace-1-uid",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, actualResourceMetrics[0].Metrics[0], "k8s.namespace.phase",
		metricspb.MetricDescriptor_GAUGE_INT64, 0)
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
