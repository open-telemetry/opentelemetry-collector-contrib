// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofilesample"

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"
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
		modified func(sample pprofile.Sample, cache pcommon.Map)
	}{
		{
			name: "link_index",
			path: &pathtest.Path[TransformContext]{
				N: "link_index",
			},
			orig:   int64(42),
			newVal: int64(43),
			modified: func(sample pprofile.Sample, _ pcommon.Map) {
				sample.SetLinkIndex(43)
			},
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pprofile.Sample, cache pcommon.Map) {
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
			modified: func(_ pprofile.Sample, cache pcommon.Map) {
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
		pathWithContext.C = ctxprofilesample.Name
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

			profileSample, profile := createProfileSampleTelemetry()

			tCtx := NewTransformContext(profileSample, profile, pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			got, err := accessor.Get(t.Context(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContext(profileSample, pprofile.NewProfile(), pprofile.NewProfilesDictionary(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			err = accessor.Set(t.Context(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exProfileSample, exProfile := createProfileSampleTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exProfileSample, exCache)

			assert.Equal(t, exProfile, profile)
			assert.Equal(t, exProfileSample, profileSample)
			assert.Equal(t, exCache, testCache)
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	profile := pprofile.NewProfile()
	profile.SetDroppedAttributesCount(42)

	ctx := NewTransformContext(pprofile.NewSample(), profile, pprofile.NewProfilesDictionary(), instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

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

			got, err := accessor.Get(t.Context(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func createProfileSampleTelemetry() (pprofile.Sample, pprofile.Profile) {
	profile := pprofile.NewProfile()
	sample := profile.Samples().AppendEmpty()
	sample.SetLinkIndex(42)

	timestamps := sample.TimestampsUnixNano()
	if timestamps.Len() == 0 {
		timestamps.EnsureCapacity(1)
		timestamps.Append(1704067200) // January 1, 2024 00:00:00 UTC
	}

	values := sample.Values()
	if values.Len() == 0 {
		values.EnsureCapacity(1)
		values.Append(3)
	}

	return sample, profile
}
