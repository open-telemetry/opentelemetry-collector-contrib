// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// TestHashAttrValue_PerType walks every pcommon.ValueType and asserts that
// distinct values within the same type hash differently, and that the empty
// case returns a stable non-zero value.
func TestHashAttrValue_PerType(t *testing.T) {
	t.Run("Str", func(t *testing.T) {
		a := pcommon.NewValueStr("alpha")
		b := pcommon.NewValueStr("beta")
		assert.NotEqual(t, hashAttrValue(a), hashAttrValue(b))
		// Same value → same hash.
		assert.Equal(t, hashAttrValue(a), hashAttrValue(pcommon.NewValueStr("alpha")))
	})

	t.Run("Bool", func(t *testing.T) {
		assert.NotEqual(t, hashAttrValue(pcommon.NewValueBool(true)), hashAttrValue(pcommon.NewValueBool(false)))
	})

	t.Run("Int", func(t *testing.T) {
		assert.NotEqual(t, hashAttrValue(pcommon.NewValueInt(1)), hashAttrValue(pcommon.NewValueInt(2)))
		// Boundary values.
		assert.NotEqual(t, hashAttrValue(pcommon.NewValueInt(0)), hashAttrValue(pcommon.NewValueInt(math.MaxInt64)))
		assert.NotEqual(t, hashAttrValue(pcommon.NewValueInt(math.MinInt64)), hashAttrValue(pcommon.NewValueInt(math.MaxInt64)))
	})

	t.Run("Double", func(t *testing.T) {
		assert.NotEqual(t, hashAttrValue(pcommon.NewValueDouble(1.0)), hashAttrValue(pcommon.NewValueDouble(2.0)))
		posZero := pcommon.NewValueDouble(0.0)
		negZero := pcommon.NewValueDouble(math.Copysign(0, -1))
		assert.NotEqual(t, hashAttrValue(posZero), hashAttrValue(negZero),
			"+0.0 and -0.0 have different bit patterns so they hash differently")
		// NaN hashes deterministically for the canonical math.NaN() bit pattern.
		nan1 := hashAttrValue(pcommon.NewValueDouble(math.NaN()))
		nan2 := hashAttrValue(pcommon.NewValueDouble(math.NaN()))
		assert.Equal(t, nan1, nan2)
	})

	t.Run("Bytes", func(t *testing.T) {
		a := pcommon.NewValueBytes()
		a.Bytes().FromRaw([]byte{1, 2, 3})
		b := pcommon.NewValueBytes()
		b.Bytes().FromRaw([]byte{1, 2, 4})
		assert.NotEqual(t, hashAttrValue(a), hashAttrValue(b))

		// Empty bytes is stable and distinct from Empty type.
		empty := pcommon.NewValueBytes()
		assert.NotEqual(t, hashAttrValue(empty), hashAttrValue(pcommon.NewValueEmpty()))
	})

	t.Run("Map", func(t *testing.T) {
		a := pcommon.NewValueMap()
		a.Map().PutStr("k", "v1")
		b := pcommon.NewValueMap()
		b.Map().PutStr("k", "v2")
		assert.NotEqual(t, hashAttrValue(a), hashAttrValue(b))

		// Empty map distinct from Empty type.
		emptyMap := pcommon.NewValueMap()
		assert.NotEqual(t, hashAttrValue(emptyMap), hashAttrValue(pcommon.NewValueEmpty()))
	})

	t.Run("Slice", func(t *testing.T) {
		a := pcommon.NewValueSlice()
		a.Slice().AppendEmpty().SetStr("x")
		b := pcommon.NewValueSlice()
		b.Slice().AppendEmpty().SetStr("y")
		assert.NotEqual(t, hashAttrValue(a), hashAttrValue(b))
	})

	t.Run("Empty", func(t *testing.T) {
		empty1 := hashAttrValue(pcommon.NewValueEmpty())
		empty2 := hashAttrValue(pcommon.NewValueEmpty())
		assert.Equal(t, empty1, empty2)
	})
}

// TestHashAttrValue_CrossTypeNoCollision asserts that the type-tag prefixes
// prevent cross-type collisions between values whose raw bytes would otherwise
// match:
//   - Bool(false) / Bool(true) vs Str("\x00") / Str("\x01")
//   - Int(0) vs Bytes of eight zero bytes
//   - Bytes(nil) vs Str("")
//   - Empty type vs any of the above
func TestHashAttrValue_CrossTypeNoCollision(t *testing.T) {
	cases := []struct {
		name string
		a, b pcommon.Value
	}{
		{
			name: "Bool(false) vs Str(\"\\x00\")",
			a:    pcommon.NewValueBool(false),
			b:    pcommon.NewValueStr("\x00"),
		},
		{
			name: "Bool(true) vs Str(\"\\x01\")",
			a:    pcommon.NewValueBool(true),
			b:    pcommon.NewValueStr("\x01"),
		},
		{
			name: "Int(0) vs Bytes(8 zero bytes)",
			a:    pcommon.NewValueInt(0),
			b:    func() pcommon.Value { v := pcommon.NewValueBytes(); v.Bytes().FromRaw(make([]byte, 8)); return v }(),
		},
		{
			name: "Bytes(empty) vs Str(empty)",
			a:    pcommon.NewValueBytes(),
			b:    pcommon.NewValueStr(""),
		},
		{
			name: "Empty vs Str(empty)",
			a:    pcommon.NewValueEmpty(),
			b:    pcommon.NewValueStr(""),
		},
		{
			name: "Empty vs Int(0)",
			a:    pcommon.NewValueEmpty(),
			b:    pcommon.NewValueInt(0),
		},
		{
			name: "Int(0) vs Double(0.0)",
			a:    pcommon.NewValueInt(0),
			b:    pcommon.NewValueDouble(0.0),
		},
		{
			name: "Map(empty) vs Slice(empty)",
			a:    pcommon.NewValueMap(),
			b:    pcommon.NewValueSlice(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, hashAttrValue(tc.a), hashAttrValue(tc.b))
		})
	}
}

// TestPairHashMix_AsymmetricMix verifies that pairHashMix is non-linear enough
// that swapping values across two pairs changes the XOR-combined result —
// i.e. pairHashMix(ka, vb) ^ pairHashMix(kc, vd) must differ from
// pairHashMix(ka, vd) ^ pairHashMix(kc, vb).
func TestPairHashMix_AsymmetricMix(t *testing.T) {
	// Use four arbitrary key/value hash slots.
	ka, kc, vb, vd := uint64(0x1111111111111111), uint64(0x2222222222222222),
		uint64(0x3333333333333333), uint64(0x4444444444444444)

	original := pairHashMix(ka, vb) ^ pairHashMix(kc, vd)
	swapped := pairHashMix(ka, vd) ^ pairHashMix(kc, vb)

	assert.NotEqual(t, original, swapped,
		"swapping values across pairs must not cancel under XOR")
}
