// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collection

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMetricsStoreOperations(t *testing.T) {
	ms := metricsStore{
		metricsCache: map[types.UID][]consumerdata.MetricsData{},
	}

	updates := []struct {
		id types.UID
		rm []*resourceMetrics
	}{
		{
			id: types.UID("test-uid-1"),
			rm: []*resourceMetrics{{}, {}},
		},
		{
			id: types.UID("test-uid-2"),
			rm: []*resourceMetrics{{}},
		},
	}

	// Update metric store with metrics
	for _, u := range updates {
		ms.update(&corev1.Pod{
			ObjectMeta: v1.ObjectMeta{
				UID: u.id,
			},
		}, u.rm)
	}

	// Asset values form updates
	expectedMetricData := 0
	for _, u := range updates {
		require.NotNil(t, ms.metricsCache[u.id])
		require.True(t, len(ms.metricsCache[u.id]) == len(u.rm))
		expectedMetricData += len(u.rm)
	}
	require.Equal(t, expectedMetricData, len(ms.getMetricData()))

	// Remove non existent item
	ms.remove(&corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID: types.UID("1"),
		},
	})
	require.Equal(t, len(updates), len(ms.metricsCache))

	// Remove valid item from cache
	ms.remove(&corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID: updates[1].id,
		},
	})
	expectedMetricData -= len(updates[1].rm)
	require.Equal(t, len(updates)-1, len(ms.metricsCache))
	require.Equal(t, expectedMetricData, len(ms.getMetricData()))

}
