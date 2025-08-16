// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofilelocation // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofilelocation"

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilelocation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter_Cache(t *testing.T) {
	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(loc pprofile.Location, cache pcommon.Map)
	}{
		{
			name: "address",
			path: &pathtest.Path[TransformContext]{
				N: "address",
			},
			orig:   uint64(42),
			newVal: uint64(43),
			modified: func(loc pprofile.Location, _ pcommon.Map) {
				loc.SetAddress(43)
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pprofile.Location, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
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
			modified: func(_ pprofile.Location, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxprofilelocation.Name
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

			profileLocation := createProfileLocationTelemetry()

			tCtx := NewTransformContext(profileLocation, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContext(profileLocation, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exProfileLocation := createProfileLocationTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exProfileLocation, exCache)

			assert.Equal(t, exProfileLocation, profileLocation)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(pprofile.NewLocation(), pprofile.NewProfilesDictionary(), instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		expected any
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
			expected: "bar",
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected: "bar",
		},
		{
			name:     "instrumentation_scope",
			path:     &pathtest.Path[TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected: instrumentationScope.Name(),
		},
		{
			name:     "instrumentation_scope with context",
			path:     &pathtest.Path[TransformContext]{C: "instrumentation_scope", N: "name"},
			expected: instrumentationScope.Name(),
		},
		{
			name:     "scope",
			path:     &pathtest.Path[TransformContext]{N: "scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected: instrumentationScope.Name(),
		},
		{
			name:     "scope with context",
			path:     &pathtest.Path[TransformContext]{C: "scope", N: "name"},
			expected: instrumentationScope.Name(),
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
}

func createProfileLocationTelemetry() pprofile.Location {
	loc := pprofile.NewLocation()
	loc.SetAddress(42)
	return loc
}
