// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxscope_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestPathGetSetter(t *testing.T) {
	refIS := createInstrumentationScope()

	refISC := newTestContext(refIS)
	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")
	tests := []struct {
		name     string
		path     ottl.Path[*testContext]
		orig     any
		newVal   any
		modified func(is pcommon.InstrumentationScope)
	}{
		{
			name: "instrumentation_scope name",
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs,
			modified: func(is pcommon.InstrumentationScope) {
				newAttrs.CopyTo(is.Attributes())
			},
		},
		{
			name: "attributes raw map",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs.AsRaw(),
			modified: func(is pcommon.InstrumentationScope) {
				_ = is.Attributes().FromRaw(newAttrs.AsRaw())
			},
		},
		{
			name: "attributes string",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("slice"),
					},
					&pathtest.Key[*testContext]{
						I: ottltest.Intp(0),
					},
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("new"),
					},
					&pathtest.Key[*testContext]{
						I: ottltest.Intp(2),
					},
					&pathtest.Key[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			accessor, err := ctxscope.PathGetSetter[*testContext](tt.path)
			assert.NoError(t, err)

			is := createInstrumentationScope()

			got, err := accessor.Get(context.Background(), newTestContext(is))
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newTestContext(is), tt.newVal)
			assert.NoError(t, err)

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

func createSchemaURLItem() ctxcommon.SchemaURLItem {
	return &TestSchemaURLItem{
		schemaURL: "schema_url",
	}
}

type testContext struct {
	is            pcommon.InstrumentationScope
	schemaURLItem ctxcommon.SchemaURLItem
}

func (r *testContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return r.is
}

func (r *testContext) GetScopeSchemaURLItem() ctxcommon.SchemaURLItem {
	return r.schemaURLItem
}

func newTestContext(is pcommon.InstrumentationScope) *testContext {
	return &testContext{is: is, schemaURLItem: createSchemaURLItem()}
}
