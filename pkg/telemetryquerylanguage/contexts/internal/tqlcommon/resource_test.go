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
	newAttrs.PutString("hello", "world")

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
				resource.Attributes().PutString("str", "newVal")
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
				resource.Attributes().PutBool("bool", false)
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
				resource.Attributes().PutInt("int", 20)
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
				resource.Attributes().PutDouble("double", 2.4)
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
				resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
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
				resource.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
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
				resource.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
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
				resource.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
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
				resource.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
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
				resource.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
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
	resource.Attributes().PutString("str", "val")
	resource.Attributes().PutBool("bool", true)
	resource.Attributes().PutInt("int", 10)
	resource.Attributes().PutDouble("double", 1.2)
	resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := resource.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStringVal("one")
	arrStr.AppendEmpty().SetStringVal("two")

	arrBool := resource.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBoolVal(true)
	arrBool.AppendEmpty().SetBoolVal(false)

	arrInt := resource.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetIntVal(2)
	arrInt.AppendEmpty().SetIntVal(3)

	arrFloat := resource.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDoubleVal(1.0)
	arrFloat.AppendEmpty().SetDoubleVal(2.0)

	arrBytes := resource.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{2, 3, 4})

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
