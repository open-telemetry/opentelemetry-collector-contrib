// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlscope

import (
	"context"
	"slices"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	refIS, _ := createTelemetry()

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
		name         string
		path         ottl.Path[TransformContext]
		orig         any
		newVal       any
		modified     func(is pcommon.InstrumentationScope, resource pcommon.Resource, cache pcommon.Map)
		setStatement string
		getStatement string
	}{
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
			setStatement: `set(cache, {"temp": "value"})`,
			getStatement: `cache`,
		},
		{
			name: "cache access",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ pcommon.InstrumentationScope, _ pcommon.Resource, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
			setStatement: `set(cache["temp"], "new value")`,
			getStatement: `cache["temp"]`,
		},
		{
			name: "attributes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
			},
			orig:   refIS.Attributes(),
			newVal: newAttrs,
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				newAttrs.CopyTo(is.Attributes())
			},
			setStatement: `set(attributes, {"hello": "world"})`,
			getStatement: `attributes`,
		},
		{
			name: "attributes string",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutStr("str", "newVal")
			},
			setStatement: `set(attributes["str"], "newVal")`,
			getStatement: `attributes["str"]`,
		},
		{
			name: "attributes bool",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("bool"),
					},
				},
			},
			orig:   true,
			newVal: false,
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutBool("bool", false)
			},
			setStatement: `set(attributes["bool"], false)`,
			getStatement: `attributes["bool"]`,
		},
		{
			name: "attributes int",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("int"),
					},
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutInt("int", 20)
			},
			setStatement: `set(attributes["int"], 20)`,
			getStatement: `attributes["int"]`,
		},
		{
			name: "attributes float",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("double"),
					},
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutDouble("double", 2.4)
			},
			setStatement: `set(attributes["double"], 2.4)`,
			getStatement: `attributes["double"]`,
		},
		{
			name: "attributes bytes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("bytes"),
					},
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{2, 3, 4})
			},
			setStatement: `set(attributes["bytes"], 0x020304)`,
			getStatement: `attributes["bytes"]`,
		},
		{
			name: "attributes array string",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_str"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_str")
				return val.Slice()
			}(),
			newVal: []string{"new"},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStr("new")
			},
			setStatement: `set(attributes["arr_str"], ["new"])`,
			getStatement: `attributes["arr_str"]`,
		},
		{
			name: "attributes array bool",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_bool"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bool")
				return val.Slice()
			}(),
			newVal: []bool{false},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBool(false)
			},
			setStatement: `set(attributes["arr_bool"], [false])`,
			getStatement: `attributes["arr_bool"]`,
		},
		{
			name: "attributes array int",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_int"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_int")
				return val.Slice()
			}(),
			newVal: []int64{20},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetInt(20)
			},
			setStatement: `set(attributes["arr_int"], [20])`,
			getStatement: `attributes["arr_int"]`,
		},
		{
			name: "attributes array float",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_float"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_float")
				return val.Slice()
			}(),
			newVal: []float64{2.0},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDouble(2.0)
			},
			setStatement: `set(attributes["arr_float"], [2.0])`,
			getStatement: `attributes["arr_float"]`,
		},
		{
			name: "attributes array bytes",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("arr_bytes"),
					},
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refIS.Attributes().Get("arr_bytes")
				return val.Slice()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytes().FromRaw([]byte{9, 6, 4})
			},
			setStatement: `set(attributes["arr_bytes"], [0x090604])`,
			getStatement: `attributes["arr_bytes"]`,
		},
		{
			name: "attributes pcommon.Map",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("pMap"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refIS.Attributes().Get("pMap")
				return val.Map()
			}(),
			newVal: newPMap,
			modified: func(il pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := il.Attributes().PutEmptyMap("pMap")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
			setStatement: `set(attributes["pMap"], {"k2": {"k1": "string"}})`,
			getStatement: `attributes["pMap"]`,
		},
		{
			name: "attributes map[string]any",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("map"),
					},
				},
			},
			orig: func() pcommon.Map {
				val, _ := refIS.Attributes().Get("map")
				return val.Map()
			}(),
			newVal: newMap,
			modified: func(il pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				m := il.Attributes().PutEmptyMap("map")
				m2 := m.PutEmptyMap("k2")
				m2.PutStr("k1", "string")
			},
			setStatement: `set(attributes["map"], {"k2": {"k1": "string"}})`,
			getStatement: `attributes["map"]`,
		},
		{
			name: "attributes nested",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("slice"),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(0),
					},
					&pathtest.Key[TransformContext]{
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
			modified: func(il pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				il.Attributes().PutEmptySlice("slice").AppendEmpty().SetEmptyMap().PutStr("map", "new")
			},
			setStatement: `set(attributes["slice"], [{"map": "new"}])`,
			getStatement: `attributes["slice"][0]["map"]`,
		},
		{
			name: "attributes nested new values",
			path: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("new"),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(2),
					},
					&pathtest.Key[TransformContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig: func() any {
				return nil
			}(),
			newVal: "new",
			modified: func(il pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				s := il.Attributes().PutEmptySlice("new")
				s.AppendEmpty()
				s.AppendEmpty()
				s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("new")
			},
			setStatement: `set(attributes["new"], [nil, nil, ["new"]])`,
			getStatement: `attributes["new"][2][0]`,
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[TransformContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.SetDroppedAttributesCount(20)
			},
			setStatement: `set(dropped_attributes_count, 20)`,
			getStatement: `dropped_attributes_count`,
		},
		{
			name: "name",
			path: &pathtest.Path[TransformContext]{
				N: "name",
			},
			orig:   refIS.Name(),
			newVal: "newname",
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.SetName("newname")
			},
			setStatement: `set(name, "newname")`,
			getStatement: `name`,
		},
		{
			name: "version",
			path: &pathtest.Path[TransformContext]{
				N: "version",
			},
			orig:   refIS.Version(),
			newVal: "next",
			modified: func(is pcommon.InstrumentationScope, _ pcommon.Resource, _ pcommon.Map) {
				is.SetVersion("next")
			},
			setStatement: `set(version, "next")`,
			getStatement: `version`,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ContextName
		testWithContext.path = ottl.Path[TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache := pcommon.NewMap()
			cacheGetter := func(_ TransformContext) pcommon.Map {
				return testCache
			}
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			is, res := createTelemetry()

			tCtx := NewTransformContext(is, res, pmetric.NewResourceMetrics())
			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exIs, exRes := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exIs, exRes, exCache)

			assert.Equal(t, exIs, is)
			assert.Equal(t, exRes, res)
			assert.Equal(t, exCache, testCache)
		})
	}

	stmtParser := createParser(t)

	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			if tt.setStatement != "" {
				statement, err := stmtParser.ParseStatement(tt.setStatement)
				require.NoError(t, err)

				is, res := createTelemetry()

				ctx := NewTransformContext(is, res, pmetric.NewResourceMetrics())

				_, executed, err := statement.Execute(context.Background(), ctx)
				require.NoError(t, err)
				assert.True(t, executed)

				getStatement, err := stmtParser.ParseValueExpression(tt.getStatement)
				require.NoError(t, err)

				is, res = createTelemetry()

				ctx = NewTransformContext(is, res, pmetric.NewResourceMetrics())

				getResult, err := getStatement.Eval(context.Background(), ctx)

				assert.NoError(t, err)
				assert.Equal(t, tt.orig, getResult)
			}
		})
	}
}

func createParser(t *testing.T) ottl.Parser[TransformContext] {
	settings := componenttest.NewNopTelemetrySettings()
	stmtParser, err := NewParser(ottlfuncs.StandardFuncs[TransformContext](), settings)
	require.NoError(t, err)
	return stmtParser
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("instrumentation_scope")

	ctx := NewTransformContext(scope, resource, pmetric.NewResourceMetrics())

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		expected     any
		getStatement string
	}{
		{
			name: "resource",
			path: &pathtest.Path[TransformContext]{C: "", N: "resource", NextPath: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(getCache)(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
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
