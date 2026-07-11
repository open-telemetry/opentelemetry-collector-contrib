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

func Test_stringifyAll(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("already_string", "hello")
	input.PutInt("int_val", 42)
	input.PutDouble("double_val", 3.14)
	input.PutBool("bool_val", true)
	input.PutEmptyBytes("bytes_val").FromRaw([]byte{1, 2, 3})
	m := input.PutEmptyMap("map_val")
	m.PutStr("nested", "value")
	s := input.PutEmptySlice("slice_val")
	s.AppendEmpty().SetInt(1)
	s.AppendEmpty().SetInt(2)
	input.PutEmpty("empty_val")

	tests := []struct {
		name string
		want func(pcommon.Map)
	}{
		{
			name: "stringify all non-string values",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("already_string", "hello")
				expectedMap.PutStr("int_val", "42")
				expectedMap.PutStr("double_val", "3.14")
				expectedMap.PutStr("bool_val", "true")
				expectedMap.PutStr("bytes_val", "AQID")
				expectedMap.PutStr("map_val", "{\"nested\":\"value\"}")
				expectedMap.PutStr("slice_val", "[1,2]")
				expectedMap.PutStr("empty_val", "")
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

			exprFunc := stringifyAll(target)

			_, err := exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_stringifyAll_emptyMap(t *testing.T) {
	scenarioMap := pcommon.NewMap()

	target := &ottl.StandardPMapGetSetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
			return tCtx, nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
			if v, ok := m.(pcommon.Map); ok {
				v.CopyTo(tCtx)
				return nil
			}
			return errors.New("expected pcommon.Map")
		},
	}

	exprFunc := stringifyAll(target)

	_, err := exprFunc(nil, scenarioMap)
	require.NoError(t, err)

	assert.Equal(t, pcommon.NewMap(), scenarioMap)
}

func Test_stringifyAll_bad_input(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, _ any) (pcommon.Map, error) {
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc := stringifyAll[any](target)

	_, err := exprFunc(nil, nil)
	assert.Error(t, err)
}
