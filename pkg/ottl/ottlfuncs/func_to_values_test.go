// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var nilOptional ottl.Optional[ottl.PMapGetter[any]]
var nilArrayOptional ottl.Optional[[]ottl.PMapGetter[any]]

func Test_toValues(t *testing.T) {
	multiMapGetters := ottl.NewTestingOptional[[]ottl.PMapGetter[any]](
		[]ottl.PMapGetter[any]{
			ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					m.PutBool("key2", true)
					m.PutInt("key3", 42)

					s := m.PutEmptySlice("key4")
					s.AppendEmpty().SetStr("value4")
					s.AppendEmpty().SetStr("value5")
					subArray := []any{"subArrValue1", "subArrValue2"}
					s.AppendEmpty().SetEmptySlice().FromRaw(subArray)
					return m, nil
				},
			},
			ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key6", "value6")
					depth1Map := m.PutEmptyMap("depth1Map")
					depth1Map.PutStr("depth1Key1", "depth1Value1")
					depth1Map.PutStr("depth1Key2", "depth1Value2")

					depth2Map := depth1Map.PutEmptyMap("depth2Map")
					depth2Map.PutStr("depth2Key1", "depth2Value1")
					depth2Map.PutStr("depth2Key2", "depth2Value2")
					return m, nil
				},
			},
		},
	)

	tests := []struct {
		name    string
		aMap    ottl.Optional[ottl.PMapGetter[any]]
		maps    ottl.Optional[[]ottl.PMapGetter[any]]
		depth   ottl.Optional[int64]
		wantRaw []any
	}{
		{
			name: "A single map, depth not specified",
			aMap: ottl.NewTestingOptional[ottl.PMapGetter[any]](ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					m.PutStr("key2", "value2")
					m.PutBool("key3", true)
					m.PutInt("key4", 42)
					m.PutDouble("key5", 3.14)
					s := m.PutEmptySlice("key6")
					s.AppendEmpty().SetStr("value6")
					s.AppendEmpty().SetStr("value7")
					return m, nil
				},
			}),
			maps:    nilArrayOptional,
			depth:   ottl.NewTestingOptional[int64](1),
			wantRaw: []any{"value1", "value2", true, int64(42), 3.14, []any{"value6", "value7"}},
		},
		{
			name: "A single map, depth 0",
			aMap: ottl.NewTestingOptional[ottl.PMapGetter[any]](ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					m.PutStr("key2", "value2")
					m.PutBool("key3", true)
					m.PutInt("key4", 42)
					m.PutDouble("key5", 3.14)
					childMap := m.PutEmptyMap("childMap")
					childMap.PutStr("childKey1", "childValue1")
					childMap.PutStr("childKey2", "childValue2")
					return m, nil
				},
			}),
			maps:  nilArrayOptional,
			depth: ottl.NewTestingOptional[int64](0),
			wantRaw: []any{"value1", "value2", true, int64(42), 3.14, map[string]any{
				"childKey1": "childValue1",
				"childKey2": "childValue2",
			}},
		},
		{
			name: "A single map, depth 1",
			aMap: ottl.NewTestingOptional[ottl.PMapGetter[any]](ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					m.PutStr("key2", "value2")
					m.PutBool("key3", true)
					m.PutInt("key4", 42)
					m.PutDouble("key5", 3.14)
					childMap := m.PutEmptyMap("childMap")
					childMap.PutStr("childKey1", "childValue1")
					childMap.PutStr("childKey2", "childValue2")
					s := m.PutEmptySlice("key6")
					s.AppendEmpty().SetStr("value6")
					s.AppendEmpty().SetStr("value7")
					return m, nil
				},
			}),
			maps:    nilArrayOptional,
			depth:   ottl.NewTestingOptional[int64](1),
			wantRaw: []any{"value1", "value2", true, int64(42), 3.14, "childValue1", "childValue2", []any{"value6", "value7"}},
		},
		{
			name:    "an array of maps, depth not specified",
			aMap:    nilOptional,
			maps:    multiMapGetters,
			wantRaw: []any{"value1", true, int64(42), []any{"value4", "value5", []any{"subArrValue1", "subArrValue2"}}, "value6", "depth1Value1", "depth1Value2", "depth2Value1", "depth2Value2"},
		},
		{
			name:  "an array of maps, depth 0",
			aMap:  nilOptional,
			maps:  multiMapGetters,
			depth: ottl.NewTestingOptional[int64](0),
			wantRaw: []any{"value1", true, int64(42), []any{"value4", "value5", []any{"subArrValue1", "subArrValue2"}}, "value6", map[string]any{
				"depth1Key1": "depth1Value1",
				"depth1Key2": "depth1Value2",
				"depth2Map": map[string]any{
					"depth2Key1": "depth2Value1",
					"depth2Key2": "depth2Value2",
				},
			}},
		},
		{
			name:  "an array of maps, depth 1",
			aMap:  nilOptional,
			maps:  multiMapGetters,
			depth: ottl.NewTestingOptional[int64](1),
			wantRaw: []any{"value1", true, int64(42), []any{"value4", "value5", []any{"subArrValue1", "subArrValue2"}}, "value6", "depth1Value1", "depth1Value2", map[string]any{
				"depth2Key1": "depth2Value1",
				"depth2Key2": "depth2Value2",
			}},
		},
		{
			name:    "an array of maps, depth 200",
			aMap:    nilOptional,
			maps:    multiMapGetters,
			depth:   ottl.NewTestingOptional[int64](200),
			wantRaw: []any{"value1", true, int64(42), []any{"value4", "value5", []any{"subArrValue1", "subArrValue2"}}, "value6", "depth1Value1", "depth1Value2", "depth2Value1", "depth2Value2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exprFunc, err := toValues[any](tc.aMap, tc.maps, tc.depth)
			assert.NoError(t, err)
			gotSlice, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			gotRaw := gotSlice.(pcommon.Slice).AsRaw()
			assert.ElementsMatch(t, gotRaw, tc.wantRaw)
		})
	}
}

func Test_toValues_invalidInputs(t *testing.T) {
	var nilOptional ottl.Optional[ottl.PMapGetter[any]]
	var nilArrayOptional ottl.Optional[[]ottl.PMapGetter[any]]
	singleMapGetter := ottl.NewTestingOptional[ottl.PMapGetter[any]](
		ottl.StandardPMapGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				m := pcommon.NewMap()
				m.PutStr("key1", "value1")
				return m, nil
			}})

	multiMapGetters := ottl.NewTestingOptional[[]ottl.PMapGetter[any]](
		[]ottl.PMapGetter[any]{
			ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key1", "value1")
					return m, nil
				},
			},
			ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					m.PutStr("key2", "value2")
					return m, nil
				},
			},
		},
	)

	tests := []struct {
		name  string
		aMap  ottl.Optional[ottl.PMapGetter[any]]
		maps  ottl.Optional[[]ottl.PMapGetter[any]]
		depth ottl.Optional[int64]
	}{
		{
			name: "both map and maps not provided",
			aMap: nilOptional,
			maps: nilArrayOptional,
		},
		{
			name: "both map and maps provided",
			aMap: singleMapGetter,
			maps: multiMapGetters,
		},
		{
			name: "negative depth",
			aMap: nilOptional,
			maps: nilArrayOptional,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exprFunc, err := toValues[any](tc.aMap, tc.maps, tc.depth)
			assert.Error(t, err)
			assert.Nil(t, exprFunc)
		})
	}
}
