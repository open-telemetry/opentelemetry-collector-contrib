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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

func TestScopePathGetSetter(t *testing.T) {
	refIS := createInstrumentationScope()

	newAttrs := pcommon.NewMap()
	newAttrs.UpsertString("hello", "world")
	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(is pcommon.InstrumentationScope)
	}{
		{
			name:   "instrumentation_scope",
			path:   []tql.Field{},
			orig:   refIS,
			newVal: pcommon.NewInstrumentationScope(),
			modified: func(is pcommon.InstrumentationScope) {
				pcommon.NewInstrumentationScope().CopyTo(is)
			},
		},
		{
			name: "instrumentation_scope name",
			path: []tql.Field{
				{
					Name: "name",
				},
			},
			orig:   refIS.Name(),
			newVal: "newname",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetName("newname")
			},
		},
		{
			name: "instrumentation_scope version",
			path: []tql.Field{
				{
					Name: "version",
				},
			},
			orig:   refIS.Version(),
			newVal: "next",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetVersion("next")
			},
		},
		{
			name: "attributes",
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs,
			modified: func(is pcommon.InstrumentationScope) {
				newAttrs.CopyTo(is.Attributes())
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().UpsertString("str", "newVal")
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().UpsertBool("bool", false)
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().UpsertInt("int", 20)
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().UpsertDouble("double", 2.4)
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
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
				val, _ := refIS.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(is pcommon.InstrumentationScope) {
				newArrStr := pcommon.NewValueSlice()
				newArrStr.SliceVal().AppendEmpty().SetStringVal("new")
				is.Attributes().Upsert("arr_str", newArrStr)
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
				val, _ := refIS.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(is pcommon.InstrumentationScope) {
				newArrBool := pcommon.NewValueSlice()
				newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)
				is.Attributes().Upsert("arr_bool", newArrBool)
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
				val, _ := refIS.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(is pcommon.InstrumentationScope) {
				newArrInt := pcommon.NewValueSlice()
				newArrInt.SliceVal().AppendEmpty().SetIntVal(20)
				is.Attributes().Upsert("arr_int", newArrInt)
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
				val, _ := refIS.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(is pcommon.InstrumentationScope) {
				newArrFloat := pcommon.NewValueSlice()
				newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
				is.Attributes().Upsert("arr_float", newArrFloat)
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
				val, _ := refIS.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(is pcommon.InstrumentationScope) {
				newArrBytes := pcommon.NewValueSlice()
				newArrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{9, 6, 4}))
				is.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ScopePathGetSetter(tt.path)
			assert.NoError(t, err)

			is := createInstrumentationScope()

			got := accessor.Get(newInstrumentationScopeContext(is))
			assert.Equal(t, tt.orig, got)

			accessor.Set(newInstrumentationScopeContext(is), tt.newVal)

			expectedIS := createInstrumentationScope()
			tt.modified(expectedIS)

			assert.Equal(t, expectedIS, is)
		})
	}
}

func createInstrumentationScope() pcommon.InstrumentationScope {
	is := pcommon.NewInstrumentationScope()
	is.SetName("library")
	is.SetVersion("version")

	is.Attributes().UpsertString("str", "val")
	is.Attributes().UpsertBool("bool", true)
	is.Attributes().UpsertInt("int", 10)
	is.Attributes().UpsertDouble("double", 1.2)
	is.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{1, 3, 2}))

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	is.Attributes().Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	is.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	is.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	is.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{1, 2, 3}))
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
	is.Attributes().Upsert("arr_bytes", arrBytes)

	return is
}

type instrumentationScopeContext struct {
	is pcommon.InstrumentationScope
}

func (r *instrumentationScopeContext) GetItem() interface{} {
	return nil
}

func (r *instrumentationScopeContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return r.is
}

func (r *instrumentationScopeContext) GetResource() pcommon.Resource {
	return pcommon.Resource{}
}

func newInstrumentationScopeContext(is pcommon.InstrumentationScope) tql.TransformContext {
	return &instrumentationScopeContext{is: is}
}
