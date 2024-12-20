// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package staleness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
)

func TestStaleness(t *testing.T) {
	stalenessMap := NewStaleness[int](
		1*time.Second,
		make(streams.HashMap[int]),
	)

	idA := generateStreamID(t, map[string]any{
		"aaa": "123",
	})
	idB := generateStreamID(t, map[string]any{
		"bbb": "456",
	})
	idC := generateStreamID(t, map[string]any{
		"ccc": "789",
	})
	idD := generateStreamID(t, map[string]any{
		"ddd": "024",
	})

	initialTime := time.Time{}
	timeA := initialTime.Add(2 * time.Second)
	timeB := initialTime.Add(1 * time.Second)
	timeC := initialTime.Add(3 * time.Second)
	timeD := initialTime.Add(4 * time.Second)

	valueA := 1
	valueB := 4
	valueC := 7
	valueD := 0

	// Add the values to the map
	NowFunc = func() time.Time { return timeA }
	_ = stalenessMap.Store(idA, valueA)
	NowFunc = func() time.Time { return timeB }
	_ = stalenessMap.Store(idB, valueB)
	NowFunc = func() time.Time { return timeC }
	_ = stalenessMap.Store(idC, valueC)
	NowFunc = func() time.Time { return timeD }
	_ = stalenessMap.Store(idD, valueD)

	// Set the time to 2.5s and run expire
	// This should remove B, but the others should remain
	// (now == 2.5s, B == 1s, max == 1s)
	// now > B + max
	NowFunc = func() time.Time { return initialTime.Add(2500 * time.Millisecond) }
	stalenessMap.ExpireOldEntries()
	validateStalenessMapEntries(t,
		map[identity.Stream]int{
			idA: valueA,
			idC: valueC,
			idD: valueD,
		},
		stalenessMap,
	)

	// Set the time to 4.5s and run expire
	// This should remove A and C, but D should remain
	// (now == 2.5s, A == 2s, C == 3s, max == 1s)
	// now > A + max AND now > C + max
	NowFunc = func() time.Time { return initialTime.Add(4500 * time.Millisecond) }
	stalenessMap.ExpireOldEntries()
	validateStalenessMapEntries(t,
		map[identity.Stream]int{
			idD: valueD,
		},
		stalenessMap,
	)
}

func validateStalenessMapEntries(t *testing.T, expected map[identity.Stream]int, sm *Staleness[int]) {
	actual := map[identity.Stream]int{}

	sm.Items()(func(key identity.Stream, value int) bool {
		actual[key] = value
		return true
	})
	require.Equal(t, expected, actual)
}

func TestEvict(t *testing.T) {
	now := 0
	NowFunc = func() time.Time {
		return time.Unix(int64(now), 0)
	}

	stale := NewStaleness(1*time.Minute, make(streams.HashMap[int]))

	now = 10
	idA := generateStreamID(t, map[string]any{"aaa": "123"})
	err := stale.Store(idA, 0)
	require.NoError(t, err)

	now = 20
	idB := generateStreamID(t, map[string]any{"bbb": "456"})
	err = stale.Store(idB, 1)
	require.NoError(t, err)

	require.Equal(t, 2, stale.Len())

	// nothing stale yet, must not evict
	_, ok := stale.Evict()
	require.False(t, ok)
	require.Equal(t, 2, stale.Len())

	// idA stale
	now = 71
	gone, ok := stale.Evict()
	require.True(t, ok)
	require.NotZero(t, gone)
	require.Equal(t, 1, stale.Len())

	// idB not yet stale
	_, ok = stale.Evict()
	require.False(t, ok)
	require.Equal(t, 1, stale.Len())
}
