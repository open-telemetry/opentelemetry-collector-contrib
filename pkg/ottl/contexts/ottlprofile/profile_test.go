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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
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
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(profile pprofile.Profile, cache pcommon.Map)
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
			pep := pathExpressionParser{}
			accessor, err := pep.parsePath(tt.path)
			assert.NoError(t, err)

			profile := createProfileTelemetry()

			tCtx := NewTransformContext(profile, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			got, err := accessor.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			tCtx = NewTransformContext(profile, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())
			err = accessor.Set(context.Background(), tCtx, tt.newVal)
			assert.NoError(t, err)

			exProfile := createProfileTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exProfile, exCache)

			assert.Equal(t, exProfile, profile)
			assert.Equal(t, exCache, tCtx.getCache())
		})
	}
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(pprofile.NewProfile(), instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

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
	}

	pep := pathExpressionParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pep.parsePath(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_newPathGetSetter_WithCache(t *testing.T) {
	cacheValue := pcommon.NewMap()
	cacheValue.PutStr("test", "pass")

	ctx := NewTransformContext(
		pprofile.NewProfile(),
		pcommon.NewInstrumentationScope(),
		pcommon.NewResource(),
		pprofile.NewScopeProfiles(),
		pprofile.NewResourceProfiles(),
		WithCache(&cacheValue),
	)

	assert.Equal(t, cacheValue, ctx.getCache())
}

var profileID = pprofile.ProfileID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func createProfileTelemetry() pprofile.Profile {
	profile := pprofile.NewProfile()
	profile.SetProfileID(profileID)
	profile.SetTime(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	//	profile.SetDuration(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
	return profile
}

func TestTransformContext_MarshalLogObject(_ *testing.T) {
	instrumentationScope := pcommon.NewInstrumentationScope()
	cache := pcommon.NewMap()

	logger := zap.NewExample()
	defer logger.Sync()

	rps := GenerateProfiles(1).ResourceProfiles()
	for i := range rps.Len() {
		rp := rps.At(i)
		resource := rp.Resource()
		sps := rp.ScopeProfiles()
		for j := range sps.Len() {
			sp := sps.At(j)
			profiles := sp.Profiles()
			for k := range profiles.Len() {
				profile := profiles.At(k)
				ctx := NewTransformContext(profile, instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles(), WithCache(&cache))
				logger.Info("test", zap.Object("context", ctx))
			}
		}
	}
}

var (
	profileStartTimestamp = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC))
	profileEndTimestamp   = pcommon.NewTimestampFromTime(time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC))
)

// GenerateProfiles generates dummy profiling data for tests
func GenerateProfiles(profilesCount int) pprofile.Profiles {
	td := pprofile.NewProfiles()
	initResource(td.ResourceProfiles().AppendEmpty().Resource())
	ss := td.ResourceProfiles().At(0).ScopeProfiles().AppendEmpty().Profiles()
	ss.EnsureCapacity(profilesCount)
	for i := 0; i < profilesCount; i++ {
		switch i % 2 {
		case 0:
			fillProfileOne(ss.AppendEmpty())
		case 1:
			fillProfileTwo(ss.AppendEmpty())
		}
	}
	return td
}

func initResource(r pcommon.Resource) {
	r.Attributes().PutStr("resource-attr", "resource-attr-val-1")
}

func fillProfileOne(profile pprofile.Profile) {
	profile.SetProfileID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	profile.SetTime(profileStartTimestamp)
	profile.SetDuration(profileEndTimestamp)
	profile.SetDroppedAttributesCount(1)

	profile.StringTable().Append("typeValue", "unitValue")

	st := profile.SampleType().AppendEmpty()
	st.SetTypeStrindex(0)
	st.SetUnitStrindex(1)
	st.SetAggregationTemporality(pprofile.AggregationTemporalityCumulative)

	attr := profile.AttributeTable().AppendEmpty()
	attr.SetKey("attry1key")
	attr.Value().SetStr("attr2value")

	profile.LocationTable().AppendEmpty()
	profile.LocationTable().AppendEmpty()
	profile.LocationTable().AppendEmpty()

	sample := profile.Sample().AppendEmpty()
	sample.SetLocationsStartIndex(1)
	sample.SetLocationsLength(2)
	sample.Value().Append(4)
	sample.AttributeIndices().Append(0)
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetProfileID([16]byte{0x02, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10})
	profile.SetTime(profileStartTimestamp)
	profile.SetDuration(profileEndTimestamp)

	attr := profile.AttributeTable().AppendEmpty()
	attr.SetKey("key")
	attr.Value().SetStr("value")

	sample := profile.Sample().AppendEmpty()
	sample.SetLocationsStartIndex(7)
	sample.SetLocationsLength(20)
	sample.Value().Append(9)
	sample.AttributeIndices().Append(0)
}
