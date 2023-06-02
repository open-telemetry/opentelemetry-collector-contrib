// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package replicaset

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestReplicasetMetrics(t *testing.T) {
	rs := testutils.NewReplicaSet("1")

	actualResourceMetrics := GetMetrics(rs)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 2, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.replicaset.uid":  "test-replicaset-1-uid",
			"k8s.replicaset.name": "test-replicaset-1",
			"k8s.namespace.name":  "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.replicaset.desired",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.replicaset.available",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}
