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

// TestTracker_RefreshBackoff pins the cost bound on a label sitting at its
// limit: once a refresh has measured growth within reach of the limit, the
// next refresh waits refreshBackoff inserts, so new values arriving in that
// gap are detected at its end rather than one estimate per insert.
func TestTracker_RefreshBackoff(t *testing.T) {
	const limit = 10
	tracker := newTracker()
	tracker.rotate() // the next insert takes a baseline, so the epoch starts at zero growth

	for v := range limit {
		require.False(t, tracker.insert(rotationBenchmarkHash(0, v), limit))
	}
	// A repeat brings growth within reach of the limit: refresh, still under.
	require.False(t, tracker.insert(rotationBenchmarkHash(0, 0), limit))
	require.True(t, tracker.refreshed)
	require.EqualValues(t, limit, tracker.delta())

	// One value past the limit, then repeats: no refresh until the backoff
	// has elapsed, and the drop lands exactly on that insert.
	require.False(t, tracker.insert(rotationBenchmarkHash(0, limit), limit))
	for range refreshBackoff - 2 {
		require.False(t, tracker.insert(rotationBenchmarkHash(0, 0), limit))
	}
	require.EqualValues(t, refreshBackoff-1, tracker.sinceRefresh)
	require.True(t, tracker.insert(rotationBenchmarkHash(0, 0), limit))
	require.EqualValues(t, limit+1, tracker.delta())
	// Over the limit stays over for the rest of the epoch.
	require.True(t, tracker.insert(rotationBenchmarkHash(0, 0), limit))
}
