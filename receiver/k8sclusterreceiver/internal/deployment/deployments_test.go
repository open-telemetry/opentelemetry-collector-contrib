// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestDeploymentMetrics(t *testing.T) {
	dep := testutils.NewDeployment("1")

	m := GetMetrics(receivertest.NewNopCreateSettings(), dep)

	require.Equal(t, 1, m.ResourceMetrics().Len())
	require.Equal(t, 2, m.MetricCount())

	rm := m.ResourceMetrics().At(0)
	assert.Equal(t,
		map[string]interface{}{
			"k8s.deployment.uid":      "test-deployment-1-uid",
			"k8s.deployment.name":     "test-deployment-1",
			"k8s.namespace.name":      "test-namespace",
			"opencensus.resourcetype": "k8s",
		},
		rm.Resource().Attributes().AsRaw(),
	)
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sms := rm.ScopeMetrics().At(0)
	require.Equal(t, 2, sms.Metrics().Len())
	sms.Metrics().Sort(func(a, b pmetric.Metric) bool {
		return a.Name() < b.Name()
	})
	testutils.AssertMetricInt(t, sms.Metrics().At(0), "k8s.deployment.available", pmetric.MetricTypeGauge, int64(3))
	testutils.AssertMetricInt(t, sms.Metrics().At(1), "k8s.deployment.desired", pmetric.MetricTypeGauge, int64(10))
}

func TestGoldenFile(t *testing.T) {
	dep := testutils.NewDeployment("1")
	m := GetMetrics(receivertest.NewNopCreateSettings(), dep)
	expectedFile := filepath.Join("testdata", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
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
