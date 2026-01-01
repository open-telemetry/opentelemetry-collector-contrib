// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofilestack // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofilestack"

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilestack"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter_Cache(t *testing.T) {
	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	tests := []struct {
		name     string
		path     ottl.Path[*TransformContext]
		orig     any
		newVal   any
		modified func(stack pprofile.Stack, cache pcommon.Map)
	}{
		{
			name: "location_indices",
			path: &pathtest.Path[*TransformContext]{
				N: "location_indices",
			},
			orig:   []int64{42},
			newVal: []int64{42, 43},
			modified: func(stack pprofile.Stack, _ pcommon.Map) {
				stack.LocationIndices().Append(43)
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pprofile.Stack, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: &pathtest.Path[*TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ pprofile.Stack, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[*TransformContext])
		pathWithContext.C = ctxprofilestack.Name
		testWithContext.path = ottl.Path[*TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache := pcommon.NewMap()
			cacheGetter := func(_ *TransformContext) pcommon.Map {
				return testCache
			}
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			require.NoError(t, err)

			profileStack, profile := createProfileStackTelemetry()

			tCtx := NewTransformContextPtr(pprofile.NewResourceProfiles(), pprofile.NewScopeProfiles(), profileStack, pprofile.NewSample(), profile, pprofile.NewProfilesDictionary())
			got, err := accessor.Get(t.Context(), tCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContextPtr(pprofile.NewResourceProfiles(), pprofile.NewScopeProfiles(), profileStack, pprofile.NewSample(), profile, pprofile.NewProfilesDictionary())
			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			require.NoError(t, err)

			exProfileSample, exProfile := createProfileStackTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exProfileSample, exCache)

			assert.Equal(t, exProfile, profile)
			assert.Equal(t, exProfileSample, profileStack)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resourceProfile := pprofile.NewResourceProfiles()
	resourceProfile.Resource().Attributes().PutStr("foo", "bar")

	scopeProfile := pprofile.NewScopeProfiles()
	scopeProfile.Scope().SetName("instrumentation_scope")

	profile := pprofile.NewProfile()
	profile.SetDroppedAttributesCount(42)

	ctx := NewTransformContextPtr(resourceProfile, scopeProfile, pprofile.NewStack(), pprofile.NewSample(), profile, pprofile.NewProfilesDictionary())

	tests := []struct {
		name     string
		path     ottl.Path[*TransformContext]
		expected any
	}{
		{
			name: "resource",
			path: &pathtest.Path[*TransformContext]{N: "resource", NextPath: &pathtest.Path[*TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*TransformContext]{
					&pathtest.Key[*TransformContext]{
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected: "bar",
		},
		{
			name: "resource with context",
			path: &pathtest.Path[*TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[*TransformContext]{
				&pathtest.Key[*TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected: "bar",
		},
		{
			name:     "instrumentation_scope",
			path:     &pathtest.Path[*TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: scopeProfile.Scope().Name(),
		},
		{
			name:     "instrumentation_scope with context",
			path:     &pathtest.Path[*TransformContext]{C: "instrumentation_scope", N: "name"},
			expected: scopeProfile.Scope().Name(),
		},
		{
			name:     "scope",
			path:     &pathtest.Path[*TransformContext]{N: "scope", NextPath: &pathtest.Path[*TransformContext]{N: "name"}},
			expected: scopeProfile.Scope().Name(),
		},
		{
			name:     "scope with context",
			path:     &pathtest.Path[*TransformContext]{C: "scope", N: "name"},
			expected: scopeProfile.Scope().Name(),
		},
	}

	testCache := pcommon.NewMap()
	cacheGetter := func(_ *TransformContext) pcommon.Map {
		return testCache
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func createProfileStackTelemetry() (pprofile.Stack, pprofile.Profile) {
	profile := pprofile.NewProfile()
	stack := pprofile.NewStack()

	stack.LocationIndices().Append(42)

	return stack, profile
}
