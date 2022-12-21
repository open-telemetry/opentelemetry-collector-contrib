// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottlresource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	refResource := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newMap := make(map[string]interface{})
	newMap2 := make(map[string]interface{})
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name     string
		path     []ottl.Field
		orig     interface{}
		newVal   interface{}
		modified func(resource pcommon.Resource)
	}{
		{
			name: "attributes",
			path: []ottl.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refResource.Attributes(),
			newVal: newAttrs,
			modified: func(resource pcommon.Resource) {
				newAttrs.CopyTo(resource.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("bool"),
				},
			},
			orig:   true,
			newVal: false,
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("int"),
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("double"),
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("bytes"),
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
		},
		{
			name: "attributes array bool",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
		},
		{
			name: "attributes array int",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
		},
		{
			name: "attributes array float",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
		},
		{
			name: "attributes pcommon.Map",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("pMap"),
				},
			},
			orig: func() pcommon.Map {
				val, _ := refResource.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(resource pcommon.Resource) {
				m := resource.Attributes().PutEmptyMap("pMap")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
		},
		{
			name: "attributes mpa[string]interface",
			path: []ottl.Field{
				{
					Name:   "attributes",
					MapKey: ottltest.Strp("map"),
				},
			},
			orig: func() pcommon.Map {
				val, _ := refResource.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(resource pcommon.Resource) {
				m := resource.Attributes().PutEmptyMap("map")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
		},
		{
			name: "dropped_attributes_count",
			path: []ottl.Field{
				{
					Name: "dropped_attributes_count",
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(resource pcommon.Resource) {
				resource.SetDroppedAttributesCount(20)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			resource := createTelemetry()

			got, err := accessor.Get(context.Background(), NewTransformContext(resource))
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), NewTransformContext(resource), tt.newVal)
			assert.Nil(t, err)

			exRes := createTelemetry()
			tt.modified(exRes)

			assert.Equal(t, exRes, resource)
		})
	}
}

func createTelemetry() pcommon.Resource {
	resource := pcommon.NewResource()

	resource.Attributes().PutStr("str", "val")
	resource.Attributes().PutBool("bool", true)
	resource.Attributes().PutInt("int", 10)
	resource.Attributes().PutDouble("double", 1.2)
	resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := resource.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := resource.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := resource.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := resource.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := resource.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	pMap := resource.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := resource.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	resource.SetDroppedAttributesCount(10)

	return resource
}
