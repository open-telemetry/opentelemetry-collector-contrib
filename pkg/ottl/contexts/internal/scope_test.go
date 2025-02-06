// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestScopePathGetSetter(t *testing.T) {
	refIS := createInstrumentationScope()

	refISC := newInstrumentationScopeContext(refIS)
	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")
	tests := []struct {
		name     string
		path     ottl.Path[*instrumentationScopeContext]
		orig     any
		newVal   any
		modified func(is pcommon.InstrumentationScope)
	}{
		{
			name: "instrumentation_scope name",
			path: &TestPath[*instrumentationScopeContext]{
				N: "name",
			},
			orig:   refIS.Name(),
			newVal: "newname",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetName("newname")
			},
		},
		{
			name: "instrumentation_scope version",
			path: &TestPath[*instrumentationScopeContext]{
				N: "version",
			},
			orig:   refIS.Version(),
			newVal: "next",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetVersion("next")
			},
		},
		{
			name: "instrumentation_scope schema_url",
			path: &TestPath[*instrumentationScopeContext]{
				N: "schema_url",
			},
			orig:   refISC.GetScopeSchemaURLItem().SchemaUrl(),
			newVal: "new_schema_url",
			modified: func(_ pcommon.InstrumentationScope) {
				refISC.GetScopeSchemaURLItem().SetSchemaUrl("new_schema_url")
			},
		},
		{
			name: "attributes",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs,
			modified: func(is pcommon.InstrumentationScope) {
				newAttrs.CopyTo(is.Attributes())
			},
		},
		{
			name: "attributes string",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutStr("str", "newVal")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &TestPath[*instrumentationScopeContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope) {
				is.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "attributes bool",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("bool"),
					},
				},
			},
			orig:   true,
			newVal: false,
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("double"),
					},
				},
			},
			orig:   1.2,
			newVal: 2.4,
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array empty",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_empty"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_empty")
				return val.Slice()
			}(),
			newVal: []any{},
			modified: func(_ pcommon.InstrumentationScope) {
				// no-op
			},
		},
		{
			name: "attributes array string",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_str"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(is pcommon.InstrumentationScope) {
				newArr := is.Attributes().PutEmptySlice("arr_str")
				newArr.AppendEmpty().SetStr("new")
			},
		},
		{
			name: "attributes array bool",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_bool"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(is pcommon.InstrumentationScope) {
				newArr := is.Attributes().PutEmptySlice("arr_bool")
				newArr.AppendEmpty().SetBool(false)
			},
		},
		{
			name: "attributes array int",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_int"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(is pcommon.InstrumentationScope) {
				newArr := is.Attributes().PutEmptySlice("arr_int")
				newArr.AppendEmpty().SetInt(20)
			},
		},
		{
			name: "attributes array float",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_float"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(is pcommon.InstrumentationScope) {
				newArr := is.Attributes().PutEmptySlice("arr_float")
				newArr.AppendEmpty().SetDouble(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("arr_bytes"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(is pcommon.InstrumentationScope) {
				newArr := is.Attributes().PutEmptySlice("arr_bytes")
				newArr.AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
		},
		{
			name: "attributes nested",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("slice"),
					},
					&TestKey[*instrumentationScopeContext]{
						I: ottltest.Intp(0),
					},
					&TestKey[*instrumentationScopeContext]{
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
			modified: func(is pcommon.InstrumentationScope) {
				is.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
		},
		{
			name: "attributes nested new values",
			path: &TestPath[*instrumentationScopeContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*instrumentationScopeContext]{
					&TestKey[*instrumentationScopeContext]{
						S: ottltest.Strp("new"),
					},
					&TestKey[*instrumentationScopeContext]{
						I: ottltest.Intp(2),
					},
					&TestKey[*instrumentationScopeContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(is pcommon.InstrumentationScope) {
				s := is.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
		},
		{
			name: "scope with context",
			path: &TestPath[*instrumentationScopeContext]{
				C: "scope",
				N: "name",
			},
			orig:   refIS.Name(),
			newVal: "newname",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetName("newname")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ScopePathGetSetter[*instrumentationScopeContext](tt.path.Context(), tt.path)
			assert.NoError(t, err)

			is := createInstrumentationScope()

			got, err := accessor.Get(context.Background(), newInstrumentationScopeContext(is))
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newInstrumentationScopeContext(is), tt.newVal)
			assert.NoError(t, err)

			expectedIS := createInstrumentationScope()
			tt.modified(expectedIS)

			assert.Equal(t, expectedIS, is)
		})
	}
}

func TestScopePathGetSetterCacheAccessError(t *testing.T) {
	path := &TestPath[*instrumentationScopeContext]{
		N: "cache",
		C: "instrumentation_scope",
		KeySlice: []ottl.Key[*instrumentationScopeContext]{
			&TestKey[*instrumentationScopeContext]{
				S: ottltest.Strp("key"),
			},
		},
		FullPath: "instrumentation_scope.cache[key]",
	}

	_, err := ScopePathGetSetter[*instrumentationScopeContext]("metric", path)
	require.Error(t, err)
	require.Contains(t, err.Error(), `replace "instrumentation_scope.cache[key]" with "metric.cache[key]"`)
}

func createInstrumentationScope() pcommon.InstrumentationScope {
	is := pcommon.NewInstrumentationScope()
	is.SetName("library")
	is.SetVersion("version")
	is.SetDroppedAttributesCount(10)

	is.Attributes().PutStr("str", "val")
	is.Attributes().PutBool("bool", true)
	is.Attributes().PutInt("int", 10)
	is.Attributes().PutDouble("double", 1.2)
	is.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	is.Attributes().PutEmptySlice("arr_empty")

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

	s := is.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	return is
}

type TestSchemaURLItem struct {
	schemaURL string
}

//revive:disable:var-naming This must implement the SchemaURL interface.
func (t *TestSchemaURLItem) SchemaUrl() string {
	return t.schemaURL
}

func (t *TestSchemaURLItem) SetSchemaUrl(v string) {
	t.schemaURL = v
}

//revive:enable:var-naming

func createSchemaURLItem() SchemaURLItem {
	return &TestSchemaURLItem{
		schemaURL: "schema_url",
	}
}

type instrumentationScopeContext struct {
	is            pcommon.InstrumentationScope
	schemaURLItem SchemaURLItem
}

func (r *instrumentationScopeContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return r.is
}

func (r *instrumentationScopeContext) GetScopeSchemaURLItem() SchemaURLItem {
	return r.schemaURLItem
}

func newInstrumentationScopeContext(is pcommon.InstrumentationScope) *instrumentationScopeContext {
	return &instrumentationScopeContext{is: is, schemaURLItem: createSchemaURLItem()}
}
