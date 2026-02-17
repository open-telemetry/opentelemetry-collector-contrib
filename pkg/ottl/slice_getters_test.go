// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_StandardStringSliceGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardStringSliceGetter[any]
		want             []string
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "[]string type",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"a", "b"}, nil
				},
			},
			want:  []string{"a", "b"},
			valid: true,
		},
		{
			name: "[]any type",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"a", "b"}, nil
				},
			},
			want:  []string{"a", "b"},
			valid: true,
		},
		{
			name: "pcommon.Slice type",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					s.AppendEmpty().SetStr("a")
					s.AppendEmpty().SetStr("b")
					return s, nil
				},
			},
			want:  []string{"a", "b"},
			valid: true,
		},
		{
			name: "invalid element type",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"a", int64(1)}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "element 1: expected string but got Int",
		},
		{
			name: "invalid target type",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Slice but got bool",
		},
		{
			name: "nil",
			getter: StandardStringSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Slice but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(t.Context(), nil)
			if tt.valid {
				require.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				var typeErr TypeError
				assert.ErrorAs(t, err, &typeErr)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardStringLikeSliceGetter(t *testing.T) {
	tests := []struct {
		name                 string
		getter               StandardStringLikeSliceGetter[any]
		want                 []*string
		valid                bool
		expectedErrorMsg     string
		expectedErrorContain string
		wantTypeError        bool
	}{
		{
			name: "[]string type",
			getter: StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"a", "b"}, nil
				},
			},
			want: []*string{
				ottltest.Strp("a"),
				ottltest.Strp("b"),
			},
			valid: true,
		},
		{
			name: "mixed []any type",
			getter: StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"a", int64(2), true, nil, []byte{0x0a}}, nil
				},
			},
			want: []*string{
				ottltest.Strp("a"),
				ottltest.Strp("2"),
				ottltest.Strp("true"),
				nil,
				ottltest.Strp("0a"),
			},
			valid: true,
		},
		{
			name: "pcommon.Value slice with bytes element",
			getter: StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueSlice()
					v.Slice().AppendEmpty().SetEmptyBytes().FromRaw([]byte{0x0a})
					return v, nil
				},
			},
			want: []*string{
				ottltest.Strp("0a"),
			},
			valid: true,
		},
		{
			name: "invalid element type in []any input",
			getter: StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"a", make(chan int)}, nil
				},
			},
			valid:                false,
			expectedErrorContain: "element 1",
			wantTypeError:        false,
		},
		{
			name: "invalid target type",
			getter: StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Slice but got bool",
			wantTypeError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(t.Context(), nil)
			if tt.valid {
				require.NoError(t, err)
				require.Len(t, val, len(tt.want))
				for i := range val {
					if tt.want[i] == nil {
						assert.Nil(t, val[i])
					} else {
						require.NotNil(t, val[i])
						assert.Equal(t, *tt.want[i], *val[i])
					}
				}
				return
			}

			require.Error(t, err)
			if tt.expectedErrorMsg != "" {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
			if tt.expectedErrorContain != "" {
				assert.ErrorContains(t, err, tt.expectedErrorContain)
			}
			if tt.wantTypeError {
				var typeErr TypeError
				assert.ErrorAs(t, err, &typeErr)
			}
		})
	}
}
