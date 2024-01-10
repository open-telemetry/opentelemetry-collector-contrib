// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestResourcePathGetSetter(t *testing.T) {
	refResource := createResource()

	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	tests := []struct {
		name     string
		path     ottl.Path[*resourceContext]
		orig     any
		newVal   any
		modified func(resource pcommon.Resource)
	}{
		{
			name:   "resource",
			path:   nil,
			orig:   refResource,
			newVal: pcommon.NewResource(),
			modified: func(resource pcommon.Resource) {
				pcommon.NewResource().CopyTo(resource)
			},
		},
		{
			name: "attributes",
			path: &TestPath[*resourceContext]{
				N: "attributes",
			},
			orig:   refResource.Attributes(),
			newVal: newAttrs,
			modified: func(resource pcommon.Resource) {
				newAttrs.CopyTo(resource.Attributes())
			},
		},
		{
			name: "attributes string",
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("str"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("bool"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("int"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("double"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array empty",
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_empty"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refResource.Attributes().Get("arr_empty")
				return val.Slice()
			}(),
			newVal: []any{},
			modified: func(resource pcommon.Resource) {
				// no-op
			},
		},
		{
			name: "attributes array string",
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_str"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_bool"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_int"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_float"),
					},
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
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("arr_bytes"),
					},
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
			name: "attributes nested",
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("slice"),
					},
					&TestKey[*resourceContext]{
						I: ottltest.Intp(0),
					},
					&TestKey[*resourceContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() string {
				val, _ := refResource.Attributes().Get("slice")
				val, _ = val.Slice().At(0).Map().Get("map")
				return val.Str()
			}(),
			newVal: "new",
			modified: func(resource pcommon.Resource) {
				resource.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
		},
		{
			name: "attributes nested new values",
			path: &TestPath[*resourceContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*resourceContext]{
					&TestKey[*resourceContext]{
						S: ottltest.Strp("new"),
					},
					&TestKey[*resourceContext]{
						I: ottltest.Intp(2),
					},
					&TestKey[*resourceContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(resource pcommon.Resource) {
				s := resource.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &TestPath[*resourceContext]{
				N: "dropped_attributes_count",
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
			accessor, err := ResourcePathGetSetter[*resourceContext](tt.path)
			assert.NoError(t, err)

			resource := createResource()

			got, err := accessor.Get(context.Background(), newResourceContext(resource))
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newResourceContext(resource), tt.newVal)
			assert.Nil(t, err)

			expectedResource := createResource()
			tt.modified(expectedResource)

			assert.Equal(t, expectedResource, resource)
		})
	}
}

func createResource() pcommon.Resource {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("str", "val")
	resource.Attributes().PutBool("bool", true)
	resource.Attributes().PutInt("int", 10)
	resource.Attributes().PutDouble("double", 1.2)
	resource.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	resource.Attributes().PutEmptySlice("arr_empty")

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

	s := resource.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	resource.SetDroppedAttributesCount(10)

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	return resource
}

type resourceContext struct {
	resource pcommon.Resource
}

func (r *resourceContext) GetResource() pcommon.Resource {
	return r.resource
}

func newResourceContext(resource pcommon.Resource) *resourceContext {
	return &resourceContext{resource: resource}
}
