// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deadlineID makes a distinguishable subtraceID for these tests.
func deadlineID(n byte) subtraceID {
	return subtraceID{traceID: makeTraceID(n), serviceID: [16]byte{0x73, 0x76, 0x63}} // "svc"
}

func TestSubtraceDeadlines_PopsInDueOrder(t *testing.T) {
	base := time.Now()
	d := newSubtraceDeadlines()

	// Inserted out of order, so the ordering can't come from insertion order.
	for _, offset := range []int{40, 10, 50, 20, 30} {
		d.set(deadlineID(byte(offset)), base.Add(time.Duration(offset)*time.Millisecond))
	}
	require.Equal(t, 5, d.len())

	next, ok := d.next()
	require.True(t, ok)
	assert.Equal(t, base.Add(10*time.Millisecond), next)

	assert.Equal(t, []subtraceID{deadlineID(10), deadlineID(20), deadlineID(30)},
		d.popDue(base.Add(35*time.Millisecond)))
	assert.Equal(t, 2, d.len())

	assert.Equal(t, []subtraceID{deadlineID(40), deadlineID(50)},
		d.popDue(base.Add(time.Second)))
	assert.Equal(t, 0, d.len())

	_, ok = d.next()
	assert.False(t, ok)
	assert.Empty(t, d.popDue(base.Add(time.Hour)))
}

// A deadline exactly at the cutoff is due: it has waited out its full duration.
func TestSubtraceDeadlines_PopsBoundaryDeadline(t *testing.T) {
	base := time.Now()
	d := newSubtraceDeadlines()
	d.set(deadlineID(1), base)

	assert.Equal(t, []subtraceID{deadlineID(1)}, d.popDue(base))
}

func TestSubtraceDeadlines_SetReplacesRatherThanDuplicates(t *testing.T) {
	base := time.Now()
	d := newSubtraceDeadlines()
	d.set(deadlineID(1), base.Add(time.Millisecond))
	d.set(deadlineID(2), base.Add(10*time.Millisecond))

	// Pushed out past the other entry, and then back in front of it.
	d.set(deadlineID(1), base.Add(time.Minute))
	require.Equal(t, 2, d.len())
	assert.Equal(t, []subtraceID{deadlineID(2)}, d.popDue(base.Add(time.Second)))

	d.set(deadlineID(1), base)
	assert.Equal(t, 1, d.len())
	assert.Equal(t, []subtraceID{deadlineID(1)}, d.popDue(base.Add(time.Second)))
}

func TestSubtraceDeadlines_Remove(t *testing.T) {
	base := time.Now()
	d := newSubtraceDeadlines()
	for i := byte(1); i <= 5; i++ {
		d.set(deadlineID(i), base.Add(time.Duration(i)*time.Millisecond))
	}

	d.remove(deadlineID(1)) // the earliest
	d.remove(deadlineID(3)) // one in the middle
	d.remove(deadlineID(5)) // the latest
	d.remove(deadlineID(9)) // never held, must be a no-op

	assert.Equal(t, 2, d.len())
	assert.Equal(t, []subtraceID{deadlineID(2), deadlineID(4)}, d.popDue(base.Add(time.Second)))
}

// The heap's ordering has to survive an arbitrary mix of operations, since a
// worker interleaves them as batches arrive and subtraces are evicted.
func TestSubtraceDeadlines_OrderingUnderRandomOperations(t *testing.T) {
	base := time.Now()
	rng := rand.New(rand.NewPCG(1, 2))
	d := newSubtraceDeadlines()
	want := map[subtraceID]time.Time{}

	for range 2000 {
		id := deadlineID(byte(rng.IntN(32)))
		switch rng.IntN(3) {
		case 0, 1:
			due := base.Add(time.Duration(rng.IntN(1000)) * time.Millisecond)
			d.set(id, due)
			want[id] = due
		default:
			d.remove(id)
			delete(want, id)
		}
		require.Equal(t, len(want), d.len())
	}

	var last time.Time
	for _, id := range d.popDue(base.Add(time.Hour)) {
		due, held := want[id]
		require.True(t, held, "popped a subtrace that had been removed")
		assert.False(t, due.Before(last), "deadlines came out of order")
		last = due
		delete(want, id)
	}
	assert.Empty(t, want, "some deadlines were never popped")
	assert.Equal(t, 0, d.len())
}
