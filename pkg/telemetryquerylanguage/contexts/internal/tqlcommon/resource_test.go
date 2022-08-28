// Copyright  The OpenTelemetry Authors
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

package tqlcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func TestResourcePathGetSetter(t *testing.T) {
	refResource := createResource()

	newAttrs := pcommon.NewMap()
	newAttrs.UpsertString("hello", "world")

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(resource pcommon.Resource)
	}{
		{
			name:   "resource",
			path:   []tql.Field{},
			orig:   refResource,
			newVal: pcommon.NewResource(),
			modified: func(resource pcommon.Resource) {
				pcommon.NewResource().CopyTo(resource)
			},
		},
		{
			name: "attributes",
			path: []tql.Field{
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
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(resource pcommon.Resource) {
				resource.Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
				},
			},
			orig:   true,
			newVal: false,
			modified: func(resource pcommon.Resource) {
				resource.Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(resource pcommon.Resource) {
				resource.Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
				},
			},
			orig:   1.2,
			newVal: 2.4,
			modified: func(resource pcommon.Resource) {
				resource.Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
			},
		},
		{
			name: "attributes array string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(resource pcommon.Resource) {
				newArrStr := pcommon.NewValueSlice()
				newArrStr.SliceVal().AppendEmpty().SetStringVal("new")
				resource.Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(resource pcommon.Resource) {
				newArrBool := pcommon.NewValueSlice()
				newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)
				resource.Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(resource pcommon.Resource) {
				newArrInt := pcommon.NewValueSlice()
				newArrInt.SliceVal().AppendEmpty().SetIntVal(20)
				resource.Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(resource pcommon.Resource) {
				newArrFloat := pcommon.NewValueSlice()
				newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
				resource.Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(resource pcommon.Resource) {
				newArrBytes := pcommon.NewValueSlice()
				newArrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{9, 6, 4}))
				resource.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
		{
			name: "dropped_attributes_count",
			path: []tql.Field{
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
			accessor, err := ResourcePathGetSetter(tt.path)
			assert.NoError(t, err)

			resource := createResource()

			got := accessor.Get(newResourceContext(resource))
			assert.Equal(t, tt.orig, got)

			accessor.Set(newResourceContext(resource), tt.newVal)

			expectedResource := createResource()
			tt.modified(expectedResource)

			assert.Equal(t, expectedResource, resource)
		})
	}
}

func createResource() pcommon.Resource {
	resource := pcommon.NewResource()
	resource.Attributes().UpsertString("str", "val")
	resource.Attributes().UpsertBool("bool", true)
	resource.Attributes().UpsertInt("int", 10)
	resource.Attributes().UpsertDouble("double", 1.2)
	resource.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{1, 3, 2}))

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	resource.Attributes().Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	resource.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	resource.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	resource.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{1, 2, 3}))
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
	resource.Attributes().Upsert("arr_bytes", arrBytes)

	resource.SetDroppedAttributesCount(10)

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	return resource
}

type resourceContext struct {
	resource pcommon.Resource
}

func (r *resourceContext) GetItem() interface{} {
	return nil
}

func (r *resourceContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return pcommon.InstrumentationScope{}
}

func (r *resourceContext) GetResource() pcommon.Resource {
	return r.resource
}

func newResourceContext(resource pcommon.Resource) tql.TransformContext {
	return &resourceContext{resource: resource}
}
