package staleness

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// RawMap
var _ Map[time.Time] = (*RawMap[identity.Stream, time.Time])(nil)

type RawMap[K comparable, V any] map[K]V

func (rm *RawMap[K, V]) Load(key K) (V, bool) {
	value, ok := (*rm)[key]
	return value, ok
}

func (rm *RawMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	returnedVal, ok := (*rm)[key]
	if !ok {
		(*rm)[key] = value
		returnedVal = value
	}

	return returnedVal, ok
}

func (rm *RawMap[K, V]) Store(key K, value V) {
	(*rm)[key] = value
}

func (rm *RawMap[K, V]) Delete(key K) {
	delete(*rm, key)
}

func (rm *RawMap[K, V]) Items() func(yield func(K, V) bool) bool {
	return func(yield func(K, V) bool) bool {
		for k, v := range *rm {
			if !yield(k, v) {
				break
			}
		}
		return false
	}
}

// Tests

func TestStaleness(t *testing.T) {
	t.Parallel()

	max := 1 * time.Second
	stalenessMap := NewStaleness[int](
		max,
		&RawMap[identity.Stream, int]{},
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
	nowFunc = func() time.Time { return timeA }
	stalenessMap.Store(idA, valueA)
	nowFunc = func() time.Time { return timeB }
	stalenessMap.Store(idB, valueB)
	nowFunc = func() time.Time { return timeC }
	stalenessMap.Store(idC, valueC)
	nowFunc = func() time.Time { return timeD }
	stalenessMap.Store(idD, valueD)

	// Set the time to 2.5s and run expire
	// This should remove B, but the others should remain
	// (now == 2.5s, B == 1s, max == 1s)
	// now > B + max
	nowFunc = func() time.Time { return initialTime.Add(2500 * time.Millisecond) }
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
	nowFunc = func() time.Time { return initialTime.Add(4500 * time.Millisecond) }
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
