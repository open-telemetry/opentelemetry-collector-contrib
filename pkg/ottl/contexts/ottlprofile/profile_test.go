// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofile

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	newAttrs := pcommon.NewMap()
	newAttrs.PutStr("hello", "world")

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newPMap := pcommon.NewMap()
	pMap2 := newPMap.PutEmptyMap("k2")
	pMap2.PutStr("k1", "string")

	newBodyMap := pcommon.NewMap()
	newBodyMap.PutStr("new", "value")

	newBodySlice := pcommon.NewSlice()
	newBodySlice.AppendEmpty().SetStr("data")

	newMap := make(map[string]any)
	newMap2 := make(map[string]any)
	newMap2["k1"] = "string"
	newMap["k2"] = newMap2

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		orig         any
		newVal       any
		modified     func(profile pprofile.Profile, cache pcommon.Map)
		setStatement string
		getStatement string
	}{
		{
			name: "time",
			path: &pathtest.Path[TransformContext]{
				N: "time",
			},
			orig:   time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC),
			newVal: time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC),
			modified: func(profile pprofile.Profile, _ pcommon.Map) {
				profile.SetTime(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: `set(time, Time("1970-01-01T00:00:00.2Z", "%Y-%m-%dT%H:%M:%S.%f%z"))`,
			getStatement: `time`,
		},
		{
			name: "time_unix_nano",
			path: &pathtest.Path[TransformContext]{
				N: "time_unix_nano",
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(profile pprofile.Profile, _ pcommon.Map) {
				profile.SetTime(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
			setStatement: "set(time_unix_nano, 200000000)",
			getStatement: "time_unix_nano",
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pprofile.Profile, cache pcommon.Map) {
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
			modified: func(_ pprofile.Profile, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
			setStatement: `set(cache["temp"], "new value")`,
			getStatement: `cache["temp"]`,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxprofile.Name
		testWithContext.path = &pathWithContext
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

			profile := createProfileTelemetry()

			tCtx := NewTransformContext(profile, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContext(profile, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exProfile := createProfileTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exProfile, exCache)

			assert.Equal(t, exProfile, profile)
			assert.Equal(t, exCache, testCache)
		})
	}

	stmtParser := createParser(t)
	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			if tt.setStatement != "" {
				statement, err := stmtParser.ParseStatement(tt.setStatement)
				require.NoError(t, err)

				profile := createProfileTelemetry()

				ctx := NewTransformContext(profile, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

				_, executed, err := statement.Execute(context.Background(), ctx)
				require.NoError(t, err)
				assert.True(t, executed)

				getStatement, err := stmtParser.ParseValueExpression(tt.getStatement)
				require.NoError(t, err)

				profile = createProfileTelemetry()

				ctx = NewTransformContext(profile, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

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

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(pprofile.NewProfile(), pprofile.NewProfilesDictionary(), instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		expected     any
		getStatement string
	}{
		{
			name: "resource",
			path: &pathtest.Path[TransformContext]{N: "resource", NextPath: &pathtest.Path[TransformContext]{
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
		{
			name:         "instrumentation_scope",
			path:         &pathtest.Path[TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
		{
			name:         "instrumentation_scope with context",
			path:         &pathtest.Path[TransformContext]{C: "instrumentation_scope", N: "name"},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
	}

	testCache := pcommon.NewMap()
	cacheGetter := func(_ TransformContext) pcommon.Map {
		return testCache
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
	stmtParser := createParser(t)
	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			getExpression, err := stmtParser.ParseValueExpression(tt.getStatement)
			require.NoError(t, err)
			require.NotNil(t, getExpression)
			getResult, err := getExpression.Eval(context.Background(), ctx)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, getResult)
		})
	}
}

var profileID = pprofile.ProfileID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func createProfileTelemetry() pprofile.Profile {
	profile := pprofile.NewProfile()
	profile.SetProfileID(profileID)
	profile.SetTime(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	return profile
}
