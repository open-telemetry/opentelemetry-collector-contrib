// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection

import (
	"testing"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
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
		rm []*agentmetricspb.ExportMetricsServiceRequest
	}{
		{
			id: types.UID("test-uid-1"),
			rm: []*agentmetricspb.ExportMetricsServiceRequest{
				{Resource: &resourcepb.Resource{Labels: map[string]string{"k1": "v1"}}},
				{Resource: &resourcepb.Resource{Labels: map[string]string{"k2": "v2"}}}},
		},
		{
			id: types.UID("test-uid-2"),
			rm: []*agentmetricspb.ExportMetricsServiceRequest{{Resource: &resourcepb.Resource{Labels: map[string]string{"k3": "v3"}}}},
		},
	}

	// Update metric store with metrics
	for _, u := range updates {
		require.NoError(t, ms.update(
			&corev1.Pod{ObjectMeta: v1.ObjectMeta{UID: u.id}},
			ocsToMetrics(u.rm)))
	}

	// Asset values form updates
	expectedMetricData := 0
	for _, u := range updates {
		require.Contains(t, ms.metricsCache, u.id)
		require.Equal(t, len(u.rm), ms.metricsCache[u.id].ResourceMetrics().Len())
		expectedMetricData += len(u.rm)
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
	expectedMetricData -= len(updates[1].rm)
	require.Equal(t, len(updates)-1, len(ms.metricsCache))
	require.Equal(t, expectedMetricData, ms.getMetricData(time.Now()).ResourceMetrics().Len())
}
