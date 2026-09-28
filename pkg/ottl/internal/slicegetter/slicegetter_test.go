// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slicegetter

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	literal bool
	value   any
	err     error
	gets    int
}

func (s *fakeSource) IsLiteral() bool {
	return s.literal
}

func (s *fakeSource) Get(context.Context, any) (any, error) {
	s.gets++
	return s.value, s.err
}

func (*fakeSource) Len(slice any) (int, bool) {
	items, ok := slice.([]any)
	return len(items), ok
}

func (*fakeSource) Range(slice any, yield func(item any) bool) (bool, error) {
	items, ok := slice.([]any)
	if !ok {
		return false, errors.New("expected a slice")
	}
	if items == nil {
		return false, nil
	}
	for _, item := range items {
		if !yield(item) {
			break
		}
	}
	return true, nil
}

func TestItemTypeAndSet(t *testing.T) {
	var sg SliceGetter[any, string]
	itemType, ok := ItemType(&sg)
	require.True(t, ok)
	assert.Equal(t, reflect.TypeFor[string](), itemType)

	require.NoError(t, Set(&sg, []string{"a"}))
	vals, err := sg.Get(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, vals)

	require.ErrorContains(t, Set(&sg, 1), "cannot set value of type int")

	var notSliceGetter []string
	_, ok = ItemType(&notSliceGetter)
	assert.False(t, ok)
	require.Error(t, Set(&notSliceGetter, []string{"a"}))
}

func TestSet_literalSource(t *testing.T) {
	tests := []struct {
		name       string
		source     *fakeSource
		wantFolded bool
		wantLen    int
	}{
		{name: "literal", source: &fakeSource{literal: true, value: []any{"a", "b"}}, wantFolded: true, wantLen: 2},
		{name: "literal typed", source: &fakeSource{literal: true, value: []string{"a"}}, wantFolded: true, wantLen: 1},
		{name: "literal nil", source: &fakeSource{literal: true}},
		{name: "literal item mismatch", source: &fakeSource{literal: true, value: []any{"a", 1}}},
		{name: "literal range error", source: &fakeSource{literal: true, value: "a"}},
		{name: "literal get error", source: &fakeSource{literal: true, err: errors.New("boom")}},
		{name: "runtime", source: &fakeSource{value: []any{"a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sg SliceGetter[any, string]
			require.NoError(t, Set(&sg, Source[any](tt.source)))
			length, ok := sg.Len()
			assert.Equal(t, tt.wantFolded, ok)
			assert.Equal(t, tt.wantLen, length)
		})
	}
}

func TestSliceGetter_runtimeSource(t *testing.T) {
	source := &fakeSource{value: []any{"a", "b"}}
	var sg SliceGetter[any, string]
	require.NoError(t, Set(&sg, Source[any](source)))

	vals, err := sg.Get(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, vals)

	var ranged []string
	nonNil, err := sg.Range(t.Context(), nil, func(v string) bool {
		ranged = append(ranged, v)
		return false
	})
	require.NoError(t, err)
	assert.True(t, nonNil)
	assert.Equal(t, []string{"a"}, ranged)
	assert.Equal(t, 2, source.gets)

	_, ok := GetScalarLiteralValues(&sg)
	assert.False(t, ok)
	_, ok = GetLiteralValues(&sg, func(v string) (string, bool) { return v, true })
	assert.False(t, ok)
	assert.Equal(t, 2, source.gets)

	source.value = []any{"a", 1}
	_, err = sg.Get(t.Context(), nil)
	require.ErrorContains(t, err, "expected slice item of type string, got int")
	_, err = sg.Range(t.Context(), nil, func(string) bool { return true })
	require.ErrorContains(t, err, "expected slice item of type string, got int")

	source.value = nil
	vals, err = sg.Get(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, vals)
	nonNil, err = sg.Range(t.Context(), nil, func(string) bool { return true })
	require.NoError(t, err)
	assert.False(t, nonNil)
}

func TestGetLiteralValues(t *testing.T) {
	sg := NewTesting[any](true, []string{"a", "b"})
	vals, ok := GetScalarLiteralValues(sg)
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, vals)

	vals, ok = GetLiteralValues(sg, func(v string) (string, bool) { return v, true })
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, vals)

	vals, ok = GetLiteralValues(sg, func(v string) (string, bool) { return v, v == "a" })
	require.False(t, ok)
	assert.Nil(t, vals)
}

func TestNewTesting(t *testing.T) {
	for _, literal := range []bool{true, false} {
		sg := NewTesting[any](literal, []int64{1, 2})
		vals, err := sg.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2}, vals)
		_, ok := sg.Len()
		assert.Equal(t, literal, ok)
	}

	nonNil, err := NewTesting[any, int64](false, nil).Range(t.Context(), nil, func(int64) bool { return true })
	require.NoError(t, err)
	assert.False(t, nonNil)
}
