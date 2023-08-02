// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMetricsStoreOperations(t *testing.T) {
	ms := metricsStore{
		metricsCache: make(map[types.UID]pmetric.Metrics),
	}

	updates := []struct {
		id types.UID
		rm pmetric.Metrics
	}{
		{
			id: types.UID("test-uid-1"),
			rm: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				m.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("k1", "v1")
				m.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("k2", "v2")
				return m
			}(),
		},
		{
			id: types.UID("test-uid-2"),
			rm: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				m.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("k3", "v3")
				return m
			}(),
		},
	}

	// Update metric store with metrics
	for _, u := range updates {
		require.NoError(t, ms.update(&corev1.Pod{ObjectMeta: v1.ObjectMeta{UID: u.id}}, u.rm))
	}

	// Asset values form updates
	expectedMetricData := 0
	for _, u := range updates {
		require.Contains(t, ms.metricsCache, u.id)
		require.Equal(t, u.rm.ResourceMetrics().Len(), ms.metricsCache[u.id].ResourceMetrics().Len())
		expectedMetricData += u.rm.ResourceMetrics().Len()
	}
	require.Equal(t, expectedMetricData, ms.getMetricData(time.Now()).ResourceMetrics().Len())

	// Remove non existent item
	require.NoError(t, ms.remove(&corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID: "1",
		},
	}))
	require.Equal(t, len(updates), len(ms.metricsCache))

	// Remove valid item from cache
	require.NoError(t, ms.remove(&corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID: updates[1].id,
		},
	}))
	expectedMetricData -= updates[1].rm.ResourceMetrics().Len()
	require.Equal(t, len(updates)-1, len(ms.metricsCache))
	require.Equal(t, expectedMetricData, ms.getMetricData(time.Now()).ResourceMetrics().Len())
}
