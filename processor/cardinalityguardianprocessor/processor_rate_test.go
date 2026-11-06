// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
)

// TestTracker_ConstantRateDetected is the regression test for #50403: a
// constant number of new values per epoch must be enforced in every epoch,
// not only the first. With 64 new values against a limit of 50, exactly the
// 51st onward are over the limit in each epoch.
func TestTracker_ConstantRateDetected(t *testing.T) {
	const newPerEpoch, limit = 64, 50
	tracker := newTracker()

	for epoch := range 4 {
		dropped := 0
		for v := range newPerEpoch {
			if tracker.insert(rotationBenchmarkHash(epoch, v), limit) {
				dropped++
			}
		}
		require.Equal(t, newPerEpoch-limit, dropped,
			"epoch %d: 64 new values at limit 50 must drop exactly the 14 past the limit", epoch)
		require.False(t, tracker.rotate())
	}
}

// TestTracker_StableSetNotEnforced: a large but unchanging value set has zero
// growth after its first epoch and is never enforced, however small the limit.
func TestTracker_StableSetNotEnforced(t *testing.T) {
	const stableValues, limit = 1000, 10
	tracker := newTracker()

	for epoch := range 4 {
		dropped := 0
		for v := range stableValues {
			if tracker.insert(rotationBenchmarkHash(0, v), limit) {
				dropped++
			}
		}
		if epoch > 0 {
			require.Zero(t, dropped, "epoch %d: a stable set must not be enforced", epoch)
			require.InDelta(t, stableValues, tracker.cachedPrevEpoch, 2)
		}
		require.False(t, tracker.rotate())
	}
}

// TestCardinalityProcessor_ConstantRateEnforced reproduces the #50403 field
// report: a steady emitter of new values was enforced in epoch 1 and never
// again. It must be enforced in every epoch. (Epoch 1 enforcing is #50406.)
func TestCardinalityProcessor_ConstantRateEnforced(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 10,
		EpochDurationSeconds:        300, // rotation is driven manually below
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, consumertest.NewNop())
	require.NoError(t, err)
	p := proc.(*cardinalityProcessor)

	const newPerEpoch = 60
	droppedByEpoch := make([]int, 4)
	for epoch := range 4 {
		for i := range newPerEpoch {
			if p.shouldDrop("api.request", "session_id", pcommon.NewValueStr(fmt.Sprintf("e%d-%d", epoch, i))) {
				droppedByEpoch[epoch]++
			}
		}
		p.rotate()
	}

	// Epoch 0 is not asserted: its baseline is zero, which is #50406's
	// territory. Epochs 1-3 are the regression for #50403.
	for epoch := 1; epoch < 4; epoch++ {
		assert.InDelta(t, newPerEpoch-cfg.MaxCardinalityDeltaPerEpoch, droppedByEpoch[epoch], 3,
			"epoch %d: constant-rate growth must be enforced every epoch (dropped %v)", epoch, droppedByEpoch)
	}
}

// TestRefreshBackoff pins the backoff ladder.
func TestRefreshBackoff(t *testing.T) {
	for _, test := range []struct{ limit, want uint64 }{
		// Below 16 the backoff is 1: nothing passes unchecked.
		{limit: 0, want: 1},
		{limit: 1, want: 1},
		{limit: 15, want: 1},
		{limit: 16, want: 2},
		{limit: 100, want: 12},
		{limit: 511, want: 63},
		{limit: 512, want: maxRefreshBackoff},
		{limit: 10000, want: maxRefreshBackoff},
	} {
		require.Equal(t, test.want, refreshBackoff(test.limit), "limit %d", test.limit)
	}
}

// TestTracker_OvershootBoundedByBackoff checks how many new values
// pass before enforcement: at least the allowance, at most
// allowance+refreshBackoff(limit)-1. The worst cases reach it.
func TestTracker_OvershootBoundedByBackoff(t *testing.T) {
	for _, limit := range []uint64{1, 10, 100, 1000} {
		for _, interleaveRepeats := range []bool{false, true} {
			name := fmt.Sprintf("limit_%d_repeats_%t", limit, interleaveRepeats)
			t.Run(name, func(t *testing.T) {
				tracker := newTracker()
				tracker.rotate() // the next insert takes a zero baseline

				passed := 0
				for v := range int(limit) * 4 {
					if interleaveRepeats {
						// Advances sinceRefresh without growing the window.
						tracker.insert(rotationBenchmarkHash(0, 0), limit)
					}
					if !tracker.insert(rotationBenchmarkHash(1, v), limit) {
						passed++
					}
				}

				// The repeated value is new once, so it uses one.
				allowance := limit
				if interleaveRepeats {
					allowance--
				}
				require.GreaterOrEqual(t, uint64(passed), allowance,
					"only %d new values passed; the first %d must never be enforced", passed, allowance)
				bound := allowance + refreshBackoff(limit) - 1
				require.LessOrEqual(t, uint64(passed), bound,
					"%d new values passed unenforced, bound is allowance+backoff-1 = %d", passed, bound)
				if refreshBackoff(limit) == 1 {
					require.Equal(t, allowance, uint64(passed),
						"a backoff of 1 leaves no unchecked window, so the count is exact")
				}
			})
		}
	}

	// Literal values, so a wrong backoff fails the test.
	for _, test := range []struct{ limit, want uint64 }{
		{limit: 10, want: 10},
		{limit: 100, want: 111},
		{limit: 1000, want: 1063},
	} {
		limit := test.limit
		t.Run(fmt.Sprintf("worst_case_limit_%d", limit), func(t *testing.T) {
			tracker := newTracker()
			tracker.rotate() // the next insert takes a zero baseline

			passed := uint64(0)
			for v := range limit {
				require.False(t, tracker.insert(rotationBenchmarkHash(1, int(v)), limit),
					"the first %d new values are the allowance and must pass", limit)
				passed++
			}

			// A repeat forces a refresh at exactly the limit.
			require.False(t, tracker.insert(rotationBenchmarkHash(1, 0), limit),
				"a repeat is not growth and must never be enforced")
			require.Equal(t, limit, tracker.delta(),
				"the construction must land growth exactly on the limit, or this is not the worst case")

			for v := limit; !tracker.insert(rotationBenchmarkHash(1, int(v)), limit); v++ {
				passed++
				require.Less(t, passed, limit*2, "enforcement never began")
			}

			require.Equal(t, test.want, passed,
				"the backoff window here is entirely unchecked, so this is the bound exactly")
		})
	}
}

// TestTracker_RepeatsThenGrowthEnforced is the #52339 review case:
// re-sent values, a duplicate, then new values, every epoch.
func TestTracker_RepeatsThenGrowthEnforced(t *testing.T) {
	const limit, baseValues, newPerEpoch = 10, 10, 60
	// 64 repeats rules out exempting only the first 64 inserts.
	for _, repeats := range []int{baseValues, 64} {
		t.Run(fmt.Sprintf("repeats_%d", repeats), func(t *testing.T) {
			tracker := newTracker()
			for v := range baseValues {
				tracker.insert(rotationBenchmarkHash(0, v), limit)
			}
			require.False(t, tracker.rotate())

			for epoch := 1; epoch <= 3; epoch++ {
				dropped := 0
				for i := range repeats {
					tracker.insert(rotationBenchmarkHash(epoch-1, i%baseValues), limit)
				}
				tracker.insert(rotationBenchmarkHash(epoch-1, 0), limit) // duplicate
				for v := range newPerEpoch {
					if tracker.insert(rotationBenchmarkHash(epoch, v), limit) {
						dropped++
					}
				}

				// At limit 10 the backoff is 1, so exactly limit values pass.
				require.Equal(t, newPerEpoch-limit, dropped,
					"epoch %d: %d new values against limit %d must drop exactly %d",
					epoch, newPerEpoch, limit, newPerEpoch-limit)
				require.False(t, tracker.rotate())
			}
		})
	}
}

// TestCardinalityProcessor_RepeatsThenGrowthEnforced is the same case
// through shouldDrop.
func TestCardinalityProcessor_RepeatsThenGrowthEnforced(t *testing.T) {
	cfg := &Config{
		MaxCardinalityDeltaPerEpoch: 10,
		EpochDurationSeconds:        300, // rotation is driven manually below
	}
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))
	proc, err := newCardinalityProcessor(t.Context(), cfg, set, consumertest.NewNop())
	require.NoError(t, err)
	p := proc.(*cardinalityProcessor)

	const baseValues, newPerEpoch = 10, 60
	send := func(epoch, i int) bool {
		return p.shouldDrop("api.request", "session_id",
			pcommon.NewValueStr(fmt.Sprintf("e%d-%d", epoch, i)))
	}
	for i := range baseValues {
		send(0, i)
	}
	p.rotate()

	for epoch := 1; epoch <= 3; epoch++ {
		dropped := 0
		for i := range baseValues {
			send(epoch-1, i)
		}
		send(epoch-1, 0) // duplicate
		for i := range newPerEpoch {
			if send(epoch, i) {
				dropped++
			}
		}
		// The backoff is 1 at limit 10, so exactly 10 pass per epoch.
		assert.Equal(t, newPerEpoch-cfg.MaxCardinalityDeltaPerEpoch, dropped,
			"epoch %d: a steady %d new values per epoch must be enforced", epoch, newPerEpoch)
		p.rotate()
	}
}
