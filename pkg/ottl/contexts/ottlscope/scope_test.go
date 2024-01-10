// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlscope

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	refIS, refResource := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map)
	}{
		{
			name: "cache",
			path: &internal.TestPath[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: &internal.TestPath[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
		{
			name: "attributes",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs,
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				newAttrs.CopyTo(is.Attributes())
			},
		},
		{
			name: "attributes string",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("bool"),
					},
				},
			},
			orig:   true,
			newVal: false,
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("double"),
					},
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("arr_str"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
		},
		{
			name: "attributes array bool",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("arr_bool"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
		},
		{
			name: "attributes array int",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("arr_int"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
		},
		{
			name: "attributes array float",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("arr_float"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("arr_bytes"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
		},
		{
			name: "attributes pcommon.Map",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("pMap"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refIS.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				m := il.Attributes().PutEmptyMap("pMap")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
		},
		{
			name: "attributes map[string]any",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refIS.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				m := il.Attributes().PutEmptyMap("map")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
		},
		{
			name: "attributes nested",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("slice"),
					},
					&internal.TestKey[TransformContext]{
						I: ottltest.Intp(0),
					},
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() string {
				val, _ := refIS.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				il.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
		},
		{
			name: "attributes nested new values",
			path: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("new"),
					},
					&internal.TestKey[TransformContext]{
						I: ottltest.Intp(2),
					},
					&internal.TestKey[TransformContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(il pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				s := il.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &internal.TestPath[TransformContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "name",
			path: &internal.TestPath[TransformContext]{
				N: "name",
			},
			orig:   refIS.Name(),
			newVal: "newname",
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.SetName("newname")
			},
		},
		{
			name: "version",
			path: &internal.TestPath[TransformContext]{
				N: "version",
			},
			orig:   refIS.Version(),
			newVal: "next",
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				is.SetVersion("next")
			},
		},
		{
			name: "resource",
			path: &internal.TestPath[TransformContext]{
				N: "resource",
			},
			orig:   refResource,
			newVal: pcommon.NewResource(),
			modified: func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map) {
				pcommon.NewResource().CopyTo(resource)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pep := pathExpressionParser{}
			accessor, err := pep.parsePath(tt.path)
			assert.NoError(t, err)

			il, resource := createTelemetry()

			tCtx := NewTransformContext(il, resource)
			got, err := accessor.Get(context.Background(), tCtx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.Nil(t, err)

			exIl, exRes := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exIl, exRes, exCache)

			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
			assert.Equal(t, exCache, tCtx.getCache())
		})
	}
}

func createTelemetry() (pcommon.InstrumentationScope, pcommon.Resource) {
	is := pcommon.NewInstrumentationScope()
	is.SetName("library")
	is.SetVersion("version")
	is.SetDroppedAttributesCount(10)

	is.Attributes().PutStr("str", "val")
	is.Attributes().PutBool("bool", true)
	is.Attributes().PutInt("int", 10)
	is.Attributes().PutDouble("double", 1.2)
	is.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := is.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := is.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := is.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := is.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := is.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	pMap := is.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := is.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	s := is.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	resource := pcommon.NewResource()
	is.Attributes().CopyTo(resource.Attributes())

	return is, resource
}
