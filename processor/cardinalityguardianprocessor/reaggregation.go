// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// reaggregateNumberDataPoints merges NumberDataPoints that share the same
// attribute identity after attribute stripping. This resolves the Single-Writer
// violation by ensuring each unique attribute set maps to exactly one data point.
//
// Merge semantics depend on the metric type:
//   - Gauge: the data point with the latest timestamp is kept (last-value-wins).
//   - Delta Sum: values are summed together into a single data point.
//
// The function operates in-place on the data point slice and returns early with
// no allocations when no identity collisions exist (the common case when no
// attributes were stripped, or stripping didn't cause collisions).
func reaggregateNumberDataPoints(dps pmetric.NumberDataPointSlice, metricType pmetric.MetricType, isDelta bool) {
	n := dps.Len()
	if n <= 1 {
		return
	}

	// Phase 1: Compute identity hashes and detect collisions.
	// We hash the attributes of each data point to determine identity.
	// If all hashes are unique, we return early with zero data mutation.
	type dpInfo struct {
		hash  uint64
		index int
	}
	infos := make([]dpInfo, n)
	seen := make(map[uint64]int, n) // hash -> first index
	hasCollisions := false

	for i := range n {
		h := hashAttributes(dps.At(i).Attributes())
		infos[i] = dpInfo{hash: h, index: i}
		if _, exists := seen[h]; exists {
			hasCollisions = true
		} else {
			seen[h] = i
		}
	}

	if !hasCollisions {
		return
	}

	// Phase 2: Merge colliding data points.
	// For each group of data points with the same identity hash, merge them
	// according to the metric type. The "winner" (first occurrence) is kept
	// and updated; all other members of the group are marked for removal.

	// Track which indices to remove (merged into their group leader).
	remove := make(map[int]bool)

	// Reset seen to track group leaders.
	seen = make(map[uint64]int, n)

	for i := range n {
		h := infos[i].hash
		leaderIdx, exists := seen[h]
		if !exists {
			seen[h] = i
			continue
		}

		// Merge data point i into the leader.
		leader := dps.At(leaderIdx)
		current := dps.At(i)

		switch {
		case metricType == pmetric.MetricTypeGauge:
			// Gauge: last-value-wins by timestamp.
			if current.Timestamp() > leader.Timestamp() {
				// Replace leader's value and timestamp with current's.
				copyNumberValue(current, leader)
				leader.SetTimestamp(current.Timestamp())
				if current.StartTimestamp() > 0 {
					leader.SetStartTimestamp(current.StartTimestamp())
				}
			}
			// Preserve exemplars from the older data point into the winning leader.
			current.Exemplars().MoveAndAppendTo(leader.Exemplars())
		case metricType == pmetric.MetricTypeSum && isDelta:
			// Delta Sum: add values together.
			addNumberValue(current, leader)
			// Use the later end timestamp.
			if current.Timestamp() > leader.Timestamp() {
				leader.SetTimestamp(current.Timestamp())
			}
			// Use the earlier start timestamp.
			if current.StartTimestamp() < leader.StartTimestamp() {
				leader.SetStartTimestamp(current.StartTimestamp())
			}
			// Combine exemplars from both data points.
			current.Exemplars().MoveAndAppendTo(leader.Exemplars())
		}

		remove[i] = true
	}

	// Phase 3: Remove merged data points by compacting the slice.
	// RemoveIf iterates in order and removes entries for which the callback
	// returns true. We track the original index via a counter.
	idx := 0
	dps.RemoveIf(func(_ pmetric.NumberDataPoint) bool {
		shouldRemove := remove[idx]
		idx++
		return shouldRemove
	})
}

// hashAttributes produces a deterministic hash of a pcommon.Map's key-value
// pairs. The hash is order-independent: we XOR individual key-value hashes
// to ensure that {a=1, b=2} and {b=2, a=1} produce the same identity.
//
// This is used to detect data points that share the same attribute identity
// after attribute stripping.
func hashAttributes(attrs pcommon.Map) uint64 {
	if attrs.Len() == 0 {
		return 0
	}
	var combined uint64
	attrs.Range(func(k string, v pcommon.Value) bool {
		// Hash key and value together as a single unit, then XOR.
		// Using a null byte separator ensures "a"+"b" != "ab"+""
		pairHash := xxhash.Sum64String(k + "\x00" + v.AsString())
		combined ^= pairHash
		return true
	})
	return combined
}

// copyNumberValue copies the numeric value from src to dst, handling both
// int and double value types.
func copyNumberValue(src, dst pmetric.NumberDataPoint) {
	switch src.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		dst.SetIntValue(src.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		dst.SetDoubleValue(src.DoubleValue())
	}
}

// addNumberValue adds the numeric value of src into dst, handling both int
// and double value types. Mixed types are promoted to double.
func addNumberValue(src, dst pmetric.NumberDataPoint) {
	switch {
	case src.ValueType() == pmetric.NumberDataPointValueTypeInt &&
		dst.ValueType() == pmetric.NumberDataPointValueTypeInt:
		dst.SetIntValue(dst.IntValue() + src.IntValue())
	case src.ValueType() == pmetric.NumberDataPointValueTypeDouble &&
		dst.ValueType() == pmetric.NumberDataPointValueTypeDouble:
		dst.SetDoubleValue(dst.DoubleValue() + src.DoubleValue())
	default:
		// Mixed types: promote to double.
		srcVal := numberDataPointToDouble(src)
		dstVal := numberDataPointToDouble(dst)
		dst.SetDoubleValue(dstVal + srcVal)
	}
}

// numberDataPointToDouble extracts the numeric value as a float64.
func numberDataPointToDouble(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	default:
		return 0
	}
}
