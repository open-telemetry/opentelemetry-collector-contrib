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
// no allocations when no identity collisions exist (the common case).
func reaggregateNumberDataPoints(dps pmetric.NumberDataPointSlice, metricType pmetric.MetricType, isDelta bool) {
	n := dps.Len()
	if n <= 1 {
		return
	}

	// Phase 1: Compute identity hashes and detect collisions.
	type dpInfo struct {
		hash  uint64
		index int
	}
	infos := make([]dpInfo, n)
	seen := make(map[uint64]int, n) // hash -> first index
	hasCollisions := false

	for i := 0; i < n; i++ {
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

	// Phase 2: Merge colliding data points into the group leader.
	remove := make(map[int]bool)
	seen = make(map[uint64]int, n)

	for i := 0; i < n; i++ {
		h := infos[i].hash
		leaderIdx, exists := seen[h]
		if !exists {
			seen[h] = i
			continue
		}

		leader := dps.At(leaderIdx)
		current := dps.At(i)

		switch {
		case metricType == pmetric.MetricTypeGauge:
			// Gauge: last-value-wins by timestamp.
			if current.Timestamp() > leader.Timestamp() {
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
			if current.Timestamp() > leader.Timestamp() {
				leader.SetTimestamp(current.Timestamp())
			}
			if current.StartTimestamp() < leader.StartTimestamp() {
				leader.SetStartTimestamp(current.StartTimestamp())
			}
			current.Exemplars().MoveAndAppendTo(leader.Exemplars())
		}

		remove[i] = true
	}

	// Phase 3: Remove merged data points by compacting the slice.
	idx := 0
	dps.RemoveIf(func(_ pmetric.NumberDataPoint) bool {
		shouldRemove := remove[idx]
		idx++
		return shouldRemove
	})
}

// hashAttributes produces a deterministic, order-independent hash of a
// pcommon.Map's key-value pairs. A null byte separator is used to prevent
// collisions between keys and values (e.g. "a"+"b" vs "ab"+"").
func hashAttributes(attrs pcommon.Map) uint64 {
	if attrs.Len() == 0 {
		return 0
	}
	var combined uint64
	attrs.Range(func(k string, v pcommon.Value) bool {
		pairHash := xxhash.Sum64String(k + "\x00" + v.AsString())
		combined ^= pairHash
		return true
	})
	return combined
}

// copyNumberValue copies the numeric value from src to dst.
func copyNumberValue(src, dst pmetric.NumberDataPoint) {
	switch src.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		dst.SetIntValue(src.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		dst.SetDoubleValue(src.DoubleValue())
	}
}

// addNumberValue adds the numeric value of src into dst. Mixed types are promoted to double.
func addNumberValue(src, dst pmetric.NumberDataPoint) {
	switch {
	case src.ValueType() == pmetric.NumberDataPointValueTypeInt &&
		dst.ValueType() == pmetric.NumberDataPointValueTypeInt:
		dst.SetIntValue(dst.IntValue() + src.IntValue())
	case src.ValueType() == pmetric.NumberDataPointValueTypeDouble &&
		dst.ValueType() == pmetric.NumberDataPointValueTypeDouble:
		dst.SetDoubleValue(dst.DoubleValue() + src.DoubleValue())
	default:
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
