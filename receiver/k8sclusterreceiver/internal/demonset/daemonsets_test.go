// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package demonset

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestDaemonsetMetrics(t *testing.T) {
	ds := testutils.NewDaemonset("1")

	actualResourceMetrics := GetMetrics(ds)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 4, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.daemonset.uid":  "test-daemonset-1-uid",
			"k8s.daemonset.name": "test-daemonset-1",
			"k8s.namespace.name": "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.daemonset.current_scheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.daemonset.desired_scheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 5)

	testutils.AssertMetricsInt(t, rm.Metrics[2], "k8s.daemonset.misscheduled_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 1)

	testutils.AssertMetricsInt(t, rm.Metrics[3], "k8s.daemonset.ready_nodes",
		metricspb.MetricDescriptor_GAUGE_INT64, 2)
}
