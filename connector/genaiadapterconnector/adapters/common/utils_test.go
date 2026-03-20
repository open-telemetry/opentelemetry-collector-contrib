// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStr_String(t *testing.T) {
	s, ok := ParseStr("hello")
	assert.True(t, ok)
	assert.Equal(t, "hello", s)
}

func TestParseStr_Nil(t *testing.T) {
	_, ok := ParseStr(nil)
	assert.False(t, ok)
}

func TestParseStr_NullString(t *testing.T) {
	_, ok := ParseStr("null")
	assert.False(t, ok)
}

func TestParseStr_Int(t *testing.T) {
	s, ok := ParseStr(42)
	assert.True(t, ok)
	assert.Equal(t, "42", s)
}

func TestParseStr_Float(t *testing.T) {
	s, ok := ParseStr(3.14)
	assert.True(t, ok)
	assert.Equal(t, "3.14", s)
}

func TestParseStr_Slice(t *testing.T) {
	s, ok := ParseStr([]string{"a", "b"})
	assert.True(t, ok)
	assert.Equal(t, `["a","b"]`, s)
}

func TestParseStr_Map(t *testing.T) {
	s, ok := ParseStr(map[string]any{"key": "val"})
	assert.True(t, ok)
	assert.Equal(t, `{"key":"val"}`, s)
}

func TestParseInt_Int(t *testing.T) {
	i, ok := ParseInt(42)
	assert.True(t, ok)
	assert.Equal(t, int64(42), i)
}

func TestParseInt_Int64(t *testing.T) {
	i, ok := ParseInt(int64(100))
	assert.True(t, ok)
	assert.Equal(t, int64(100), i)
}

func TestParseInt_Float64(t *testing.T) {
	i, ok := ParseInt(float64(7.0))
	assert.True(t, ok)
	assert.Equal(t, int64(7), i)
}

func TestParseInt_String(t *testing.T) {
	_, ok := ParseInt("42")
	assert.False(t, ok)
}

func TestParseFloat_Float64(t *testing.T) {
	f, ok := ParseFloat(3.14)
	assert.True(t, ok)
	assert.Equal(t, 3.14, f)
}

func TestParseFloat_Int(t *testing.T) {
	f, ok := ParseFloat(42)
	assert.True(t, ok)
	assert.Equal(t, 42.0, f)
}

func TestParseFloat_Int64(t *testing.T) {
	f, ok := ParseFloat(int64(100))
	assert.True(t, ok)
	assert.Equal(t, 100.0, f)
}

func TestParseFloat_String(t *testing.T) {
	_, ok := ParseFloat("3.14")
	assert.False(t, ok)
}

func TestParseJSON_Primitives(t *testing.T) {
	assert.Equal(t, "hello", ParseJSON("hello", 5))
	assert.Equal(t, 42, ParseJSON(42, 5))
	assert.Equal(t, true, ParseJSON(true, 5))
	assert.Nil(t, ParseJSON(nil, 5))
}

func TestParseJSON_Map(t *testing.T) {
	input := map[string]any{"key": "val", "num": 1}
	result := ParseJSON(input, 5).(map[string]any)
	assert.Equal(t, "val", result["key"])
	assert.Equal(t, 1, result["num"])
}

func TestParseJSON_Slice(t *testing.T) {
	input := []any{"a", "b", "c"}
	result := ParseJSON(input, 5).([]any)
	assert.Len(t, result, 3)
	assert.Equal(t, "a", result[0])
}

func TestParseJSON_DepthLimit(t *testing.T) {
	input := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": "deep",
			},
		},
	}

	result := ParseJSON(input, 2).(map[string]any)
	l1 := result["l1"].(map[string]any)
	assert.Equal(t, "...", l1["l2"])
}

func TestParseJSON_NestedSliceDepth(t *testing.T) {
	input := []any{[]any{[]any{"deep"}}}

	result := ParseJSON(input, 2).([]any)
	inner := result[0].([]any)
	assert.Equal(t, "...", inner[0])
}
