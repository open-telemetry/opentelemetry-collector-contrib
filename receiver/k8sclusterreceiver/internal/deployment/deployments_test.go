// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestDeploymentMetrics(t *testing.T) {
	dep := testutils.NewDeployment("1")

	actualResourceMetrics := GetMetrics(dep)

	require.Equal(t, 1, len(actualResourceMetrics))
	require.Equal(t, 2, len(actualResourceMetrics[0].Metrics))

	rm := actualResourceMetrics[0]
	testutils.AssertResource(t, rm.Resource, constants.K8sType,
		map[string]string{
			"k8s.deployment.uid":  "test-deployment-1-uid",
			"k8s.deployment.name": "test-deployment-1",
			"k8s.namespace.name":  "test-namespace",
		},
	)

	testutils.AssertMetricsInt(t, rm.Metrics[0], "k8s.deployment.desired",
		metricspb.MetricDescriptor_GAUGE_INT64, 10)

	testutils.AssertMetricsInt(t, rm.Metrics[1], "k8s.deployment.available",
		metricspb.MetricDescriptor_GAUGE_INT64, 3)
}
