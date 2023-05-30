// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hpa

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHPAMetrics(t *testing.T) {
	hpa := testutils.NewHPA("1")

	actualResourceMetrics := GetMetrics(hpa)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.hpa.uid":        "test-hpa-1-uid",
			"k8s.hpa.name":       "test-hpa-1",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.hpa.max_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.hpa.min_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)

	testutils.AssertMetricsInt(t, rm.Metrics[2], "k8s.hpa.current_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.Metrics[3], "k8s.hpa.desired_replicas",
		metricspb.MetricDescriptor_GAUGE_INT64, 7)
}
