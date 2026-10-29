// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestCoerce_Int(t *testing.T) {
	target := GenAIUsageInputTokens

	t.Run("int passthrough", func(t *testing.T) {
		src := pcommon.NewValueInt(100)
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, pcommon.ValueTypeInt, dst.Type())
		assert.Equal(t, int64(100), dst.Int())
	})

	t.Run("string parsed to int", func(t *testing.T) {
		src := pcommon.NewValueStr("42")
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, pcommon.ValueTypeInt, dst.Type())
		assert.Equal(t, int64(42), dst.Int())
	})

	t.Run("integral double accepted", func(t *testing.T) {
		src := pcommon.NewValueDouble(7)
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, int64(7), dst.Int())
	})

	t.Run("non-integral double rejected", func(t *testing.T) {
		src := pcommon.NewValueDouble(3.14)
		dst := pcommon.NewValueEmpty()
		assert.False(t, Coerce(target, src, dst))
	})

	t.Run("non-numeric string rejected", func(t *testing.T) {
		src := pcommon.NewValueStr("not a number")
		dst := pcommon.NewValueEmpty()
		assert.False(t, Coerce(target, src, dst))
	})
}

func TestCoerce_Float64(t *testing.T) {
	target := GenAIRequestTemperature

	t.Run("double passthrough", func(t *testing.T) {
		src := pcommon.NewValueDouble(0.7)
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, pcommon.ValueTypeDouble, dst.Type())
		assert.Equal(t, 0.7, dst.Double())
	})

	t.Run("int promoted to float64", func(t *testing.T) {
		src := pcommon.NewValueInt(1)
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, 1.0, dst.Double())
	})

	t.Run("string parsed to float64", func(t *testing.T) {
		src := pcommon.NewValueStr("0.5")
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, 0.5, dst.Double())
	})

	t.Run("non-numeric string rejected", func(t *testing.T) {
		src := pcommon.NewValueStr("hot")
		dst := pcommon.NewValueEmpty()
		assert.False(t, Coerce(target, src, dst))
	})
}

func TestCoerce_String(t *testing.T) {
	target := GenAIRequestModel

	t.Run("string passthrough", func(t *testing.T) {
		src := pcommon.NewValueStr("gpt-4")
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, "gpt-4", dst.Str())
	})

	t.Run("scalar non-string stringified", func(t *testing.T) {
		src := pcommon.NewValueInt(42)
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		assert.Equal(t, pcommon.ValueTypeStr, dst.Type())
		assert.Equal(t, "42", dst.Str())
	})

	t.Run("structured source rejected", func(t *testing.T) {
		src := pcommon.NewValueSlice()
		src.Slice().AppendEmpty().SetStr("nope")
		dst := pcommon.NewValueEmpty()
		assert.False(t, Coerce(target, src, dst))
	})
}

func TestCoerce_StringSlice(t *testing.T) {
	target := GenAIResponseFinishReasons

	t.Run("string wrapped to single-element slice", func(t *testing.T) {
		src := pcommon.NewValueStr("stop")
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		require.Equal(t, pcommon.ValueTypeSlice, dst.Type())
		require.Equal(t, 1, dst.Slice().Len())
		assert.Equal(t, "stop", dst.Slice().At(0).Str())
	})

	t.Run("homogeneous string slice passthrough", func(t *testing.T) {
		src := pcommon.NewValueSlice()
		src.Slice().AppendEmpty().SetStr("stop")
		src.Slice().AppendEmpty().SetStr("length")
		dst := pcommon.NewValueEmpty()
		require.True(t, Coerce(target, src, dst))
		require.Equal(t, 2, dst.Slice().Len())
		assert.Equal(t, "stop", dst.Slice().At(0).Str())
		assert.Equal(t, "length", dst.Slice().At(1).Str())
	})

	t.Run("mixed-type slice rejected", func(t *testing.T) {
		src := pcommon.NewValueSlice()
		src.Slice().AppendEmpty().SetStr("stop")
		src.Slice().AppendEmpty().SetInt(42)
		dst := pcommon.NewValueEmpty()
		assert.False(t, Coerce(target, src, dst))
	})
}

func TestCoerce_UnknownTargetPassesThrough(t *testing.T) {
	// gen_ai.input.messages is "any" in the spec, no constructor in semconv.
	src := pcommon.NewValueStr(`[{"role":"user"}]`)
	dst := pcommon.NewValueEmpty()
	require.True(t, Coerce(GenAIInputMessages, src, dst))
	assert.Equal(t, pcommon.ValueTypeStr, dst.Type())
	assert.Equal(t, `[{"role":"user"}]`, dst.Str())
}
