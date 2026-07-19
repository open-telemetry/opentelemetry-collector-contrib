// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"encoding/binary"
	"math"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Per-type domain constants. XORed into the per-type hash so that, e.g.,
// Bool(true) does not collide with Str("\x01") and Int(0) does not collide
// with Bytes of eight zero bytes. Picked as independent random-looking
// 64-bit values; they don't need any cryptographic property.
const (
	tagStr    uint64 = 0x9E3779B97F4A7C15
	tagBool   uint64 = 0xBF58476D1CE4E5B9
	tagInt    uint64 = 0x94D049BB133111EB
	tagDouble uint64 = 0xC2B2AE3D27D4EB4F
	tagBytes  uint64 = 0x165667B19E3779F9
	tagMap    uint64 = 0x85EBCA77C2B2AE63
	tagSlice  uint64 = 0xCC9E2D51A3C59AC3
	tagEmpty  uint64 = 0x1B873593D2E0FCE5
)

// hashAttrValue returns a 64-bit hash for a single attribute value, dispatching
// on pcommon.ValueType so Str/Int/Double/Bool/Bytes stay on a zero-allocation
// path. Map and Slice fall back to AsString (JSON-marshal) — rare in metric
// attributes. Each branch XORs in a type-domain tag so values of different
// types never collide just because their byte representations match.
//
// This processor does not reuse aggregateutil.dataPointHashKey: it hashes a
// whole pcommon.Map via AsRaw + json.Marshal per call, which would regress
// the shouldDrop hot path (one call per attribute per data point).
func hashAttrValue(v pcommon.Value) uint64 {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return xxhash.Sum64String(v.Str()) ^ tagStr
	case pcommon.ValueTypeBool:
		if v.Bool() {
			return tagBool ^ 1
		}
		return tagBool
	case pcommon.ValueTypeInt:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v.Int()))
		return xxhash.Sum64(buf[:]) ^ tagInt
	case pcommon.ValueTypeDouble:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v.Double()))
		return xxhash.Sum64(buf[:]) ^ tagDouble
	case pcommon.ValueTypeBytes:
		return xxhash.Sum64(v.Bytes().AsRaw()) ^ tagBytes
	case pcommon.ValueTypeMap:
		return xxhash.Sum64String(v.AsString()) ^ tagMap
	case pcommon.ValueTypeSlice:
		return xxhash.Sum64String(v.AsString()) ^ tagSlice
	case pcommon.ValueTypeEmpty:
		return tagEmpty
	}
	return 0
}

// pairHashMix combines two 64-bit hashes non-linearly so that XOR-combining
// the result across pairs stays order-independent but does not cancel under
// cross-pair value swaps. The constant 0x9E3779B97F4A7C15 is the 64-bit
// golden-ratio fraction used by xxhash, boost::hash_combine, and Go's runtime.
func pairHashMix(a, b uint64) uint64 {
	a ^= b + 0x9E3779B97F4A7C15 + (a << 6) + (a >> 2)
	return a
}
