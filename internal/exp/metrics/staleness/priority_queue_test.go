// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

func TestPriorityQueueImpl(t *testing.T) {
	t.Parallel()

	pq := NewPriorityQueue()

	idA := generateStreamID(t, map[string]any{
		"aaa": "123",
	})
	idB := generateStreamID(t, map[string]any{
		"bbb": "456",
	})
	idC := generateStreamID(t, map[string]any{
		"ccc": "789",
	})

	initialTime := time.Time{}
	prioA := initialTime.Add(2 * time.Second)
	prioB := initialTime.Add(1 * time.Second)
	prioC := initialTime.Add(3 * time.Second)

	pq.Update(idA, prioA)
	pq.Update(idB, prioB)
	pq.Update(idC, prioC)

	// The first item should be B
	id, prio := pq.Peek()
	require.Equal(t, idB, id)
	require.Equal(t, prioB, prio)

	// If we peek again, nothing should change
	id, prio = pq.Peek()
	require.Equal(t, idB, id)
	require.Equal(t, prioB, prio)

	// Pop should return the same thing
	id, prio = pq.Pop()
	require.Equal(t, idB, id)
	require.Equal(t, prioB, prio)

	// Now if we peek again, it should be the next item
	id, prio = pq.Peek()
	require.Equal(t, idA, id)
	require.Equal(t, prioA, prio)

	// Pop should return the same thing
	id, prio = pq.Pop()
	require.Equal(t, idA, id)
	require.Equal(t, prioA, prio)

	// One last time
	id, prio = pq.Peek()
	require.Equal(t, idC, id)
	require.Equal(t, prioC, prio)

	// Pop should return the same thing
	id, prio = pq.Pop()
	require.Equal(t, idC, id)
	require.Equal(t, prioC, prio)

	// The queue should now be empty
	require.Equal(t, 0, pq.Len())

	// And the inner lookup map should also be empty
	require.IsType(t, &heapPriorityQueue{}, pq)
	heapQueue := pq.(*heapPriorityQueue)
	require.Empty(t, heapQueue.itemLookup)
}

func generateStreamID(t *testing.T, attributes map[string]any) identity.Stream {
	res := pcommon.NewResource()
	err := res.Attributes().FromRaw(map[string]any{
		"foo":  "bar",
		"asdf": "qwer",
	})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("TestScope")
	scope.SetVersion("v1.2.3")
	err = scope.Attributes().FromRaw(map[string]any{
		"aaa": "bbb",
		"ccc": "ddd",
	})
	require.NoError(t, err)

	metric := pmetric.NewMetric()

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(678)
	dp.SetTimestamp(789)
	dp.SetDoubleValue(123.456)
	err = dp.Attributes().FromRaw(attributes)
	require.NoError(t, err)

	return identity.OfStream(identity.OfResourceMetric(res, scope, metric), dp)
}
