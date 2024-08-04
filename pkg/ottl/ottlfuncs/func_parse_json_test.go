// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseJSON(t *testing.T) {
	tests := []struct {
		name      string
		target    ottl.StringGetter[any]
		wantMap   func(pcommon.Map)
		wantSlice func(pcommon.Slice)
	}{
		{
			name: "handle string",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":"string value"}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "string value")
			},
		},
		{
			name: "handle bool",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":true}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutBool("test", true)
			},
		},
		{
			name: "handle int",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":1}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1)
			},
		},
		{
			name: "handle float",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":1.1}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1.1)
			},
		},
		{
			name: "handle nil",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":null}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutEmpty("test")
			},
		},
		{
			name: "handle array",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":["string","value"]}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				emptySlice := expectedMap.PutEmptySlice("test")
				emptySlice.AppendEmpty().SetStr("string")
				emptySlice.AppendEmpty().SetStr("value")
			},
		},
		{
			name: "handle top level array",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `["string","value"]`, nil
				},
			},
			wantSlice: func(expectedSlice pcommon.Slice) {
				expectedSlice.AppendEmpty().SetStr("string")
				expectedSlice.AppendEmpty().SetStr("value")
			},
		},
		{
			name: "handle top level array of objects",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `[{"test":"value"},{"test":"value"}]`, nil
				},
			},
			wantSlice: func(expectedSlice pcommon.Slice) {

				expectedSlice.AppendEmpty().SetEmptyMap().PutStr("test", "value")
				expectedSlice.AppendEmpty().SetEmptyMap().PutStr("test", "value")
			},
		},
		{
			name: "handle nested object",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":{"nested":"true"}}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				newMap := expectedMap.PutEmptyMap("test")
				newMap.PutStr("nested", "true")
			},
		},
		{
			name: "updates existing",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"existing":"pass"}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("existing", "pass")
			},
		},
		{
			name: "complex",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test1":{"nested":"true"},"test2":"string","test3":1,"test4":1.1,"test5":[[1], [2, 3],[]],"test6":null}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				newMap := expectedMap.PutEmptyMap("test1")
				newMap.PutStr("nested", "true")
				expectedMap.PutStr("test2", "string")
				expectedMap.PutDouble("test3", 1)
				expectedMap.PutDouble("test4", 1.1)
				slice := expectedMap.PutEmptySlice("test5")
				slice0 := slice.AppendEmpty().SetEmptySlice()
				slice0.AppendEmpty().SetDouble(1)
				slice1 := slice.AppendEmpty().SetEmptySlice()
				slice1.AppendEmpty().SetDouble(2)
				slice1.AppendEmpty().SetDouble(3)
				slice.AppendEmpty().SetEmptySlice()
				expectedMap.PutEmpty("test6")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseJSON(tt.target)
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			if tt.wantMap != nil {
				resultMap, ok := result.(pcommon.Map)
				require.True(t, ok)
				expected := pcommon.NewMap()
				tt.wantMap(expected)
				assert.Equal(t, expected.Len(), resultMap.Len())
				expected.Range(func(k string, _ pcommon.Value) bool {
					ev, _ := expected.Get(k)
					av, _ := resultMap.Get(k)
					assert.Equal(t, ev, av)
					return true
				})
			} else if tt.wantSlice != nil {
				resultSlice, ok := result.(pcommon.Slice)
				require.True(t, ok)
				expected := pcommon.NewSlice()
				tt.wantSlice(expected)
				assert.Equal(t, expected, resultSlice)
			}

		})
	}
}

func Test_ParseJSON_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return 1, nil
		},
	}
	exprFunc := parseJSON[any](target)
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
