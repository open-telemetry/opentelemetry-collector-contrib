// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_truncateAll(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name  string
		limit int64
		want  func(pcommon.Map)
	}{
		{
			name:  "truncate map",
			limit: 1,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "h")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate map to zero",
			limit: 0,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate nothing",
			limit: 100,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate exact",
			limit: 11,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := TruncateAll(target, tt.limit)
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_truncateAll_UTF8(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		limit  int64
		expect string
	}{
		{
			name:   "mid-rune truncation backs up to boundary",
			input:  "ab😀c", // 'ab' (2) + emoji (4) + 'c' (1) = 7 bytes
			limit:  4,      // cuts inside emoji, backs up to 'ab'
			expect: "ab",
		},
		{
			name:   "exact rune boundary preserved",
			input:  "ab😀c",
			limit:  6, // exactly after emoji
			expect: "ab😀",
		},
		{
			name:   "invalid UTF-8 uses byte-level cut",
			input:  string([]byte{0x80, 0x81, 0x82, 0x83}),
			limit:  2,
			expect: string([]byte{0x80, 0x81}),
		},
		{
			// Grapheme cluster: "👩🏾‍🦳" (woman with white hair) is 1 visible character
			// but consists of 4 Unicode code points (runes):
			//   👩 (woman)           = 4 bytes (f0 9f 91 a9)
			//   🏾 (skin tone)       = 4 bytes (f0 9f 8f be)
			//   ‍ (zero-width joiner) = 3 bytes (e2 80 8d)
			//   🦳 (white hair)      = 4 bytes (f0 9f a6 b3)
			// Total: 15 bytes, 4 runes, 1 visible character
			// Truncating at limit=10 lands in the middle of the byte 9-11,
			// so we back up to byte 8 (end of skin tone modifier).
			// Result is valid UTF-8 but a split grapheme cluster.
			name:   "grapheme cluster truncates at rune boundary not grapheme boundary",
			input:  "👩🏾‍🦳",
			limit:  10,
			expect: "👩🏾",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			scenarioMap.PutStr("test", tt.input)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := TruncateAll(target, tt.limit)
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			result, _ := scenarioMap.Get("test")
			assert.Equal(t, tt.expect, result.Str())
		})
	}
}

func Test_truncateAll_validation(t *testing.T) {
	_, err := TruncateAll[any](&ottl.StandardPMapGetSetter[any]{}, -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid limit for truncate_all function, -1 cannot be negative")
}

func Test_truncateAll_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	require.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_truncateAll_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	require.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
