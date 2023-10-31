// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hpa

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHPAMetrics(t *testing.T) {
	hpa := testutils.NewHPA("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	RecordMetrics(mb, hpa, ts)
	m := mb.Emit()

	require.Equal(t, 1, m.ResourceMetrics().Len())
	rm := m.ResourceMetrics().At(0)
	assert.Equal(t,
		map[string]any{
			"k8s.hpa.uid":        "test-hpa-1-uid",
			"k8s.hpa.name":       "test-hpa-1",
			"k8s.namespace.name": "test-namespace",
		},
		rm.Resource().Attributes().AsRaw())

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sms := rm.ScopeMetrics().At(0)
	require.Equal(t, 4, sms.Metrics().Len())
	sms.Metrics().Sort(func(a, b pmetric.Metric) bool {
		return a.Name() < b.Name()
	})
	testutils.AssertMetricInt(t, sms.Metrics().At(0), "k8s.hpa.current_replicas", pmetric.MetricTypeGauge, 5)
	testutils.AssertMetricInt(t, sms.Metrics().At(1), "k8s.hpa.desired_replicas", pmetric.MetricTypeGauge, 7)
	testutils.AssertMetricInt(t, sms.Metrics().At(2), "k8s.hpa.max_replicas", pmetric.MetricTypeGauge, 10)
	testutils.AssertMetricInt(t, sms.Metrics().At(3), "k8s.hpa.min_replicas", pmetric.MetricTypeGauge, 2)
}
