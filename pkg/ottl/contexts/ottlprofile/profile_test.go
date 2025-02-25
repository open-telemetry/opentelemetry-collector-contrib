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
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
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
			path: &internal.TestPath[TransformContext]{
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
			path: &internal.TestPath[TransformContext]{
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
			path: &internal.TestPath[TransformContext]{
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
			path: &internal.TestPath[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
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
		pathWithContext := *tt.path.(*internal.TestPath[TransformContext])
		pathWithContext.C = ContextName
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
			path: &internal.TestPath[TransformContext]{N: "resource", NextPath: &internal.TestPath[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected: "bar",
		},
		{
			name: "resource with context",
			path: &internal.TestPath[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&internal.TestKey[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected: "bar",
		},
		{
			name:     "instrumentation_scope",
			path:     &internal.TestPath[TransformContext]{N: "instrumentation_scope", NextPath: &internal.TestPath[TransformContext]{N: "name"}},
			expected: instrumentationScope.Name(),
		},
		{
			name:     "instrumentation_scope with context",
			path:     &internal.TestPath[TransformContext]{C: "instrumentation_scope", N: "name"},
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

type mockObjectEncoder struct {
	fields map[string]interface{}
}

func (m *mockObjectEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	return nil
}

func (m *mockObjectEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	m.fields[key] = marshaler
	return nil
}

func (m *mockObjectEncoder) AddString(key, value string) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddInt32(key string, value int32) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddInt64(key string, value int64) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddFloat64(key string, value float64) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddBool(key string, value bool) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddDuration(key string, value time.Duration) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddTime(key string, value time.Time) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddBinary(key string, value []byte) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddReflected(key string, value interface{}) error {
	m.fields[key] = value
	return nil
}

func (m *mockObjectEncoder) OpenNamespace(key string) {
	// No-op for mock
}

func (m *mockObjectEncoder) AddByteString(key string, value []byte) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddComplex128(key string, value complex128) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddComplex64(key string, value complex64) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddFloat32(key string, value float32) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddInt(key string, value int) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddInt16(key string, value int16) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddInt8(key string, value int8) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUint(key string, value uint) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUint32(key string, value uint32) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUint64(key string, value uint64) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUint16(key string, value uint16) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUint8(key string, value uint8) {
	m.fields[key] = value
}

func (m *mockObjectEncoder) AddUintptr(key string, value uintptr) {
	m.fields[key] = value
}

func TestTransformContext_MarshalLogObject(t *testing.T) {
	profile := pprofile.NewProfile()
	profile.SetProfileID(pprofile.ProfileID{1, 2, 3, 4})
	profile.SetTime(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	profile.StringTable().Append("typeValue", "unitValue")
	st := profile.SampleType().AppendEmpty()
	st.SetTypeStrindex(0)
	st.SetUnitStrindex(1)
	st.SetAggregationTemporality(3)

	instrumentationScope := pcommon.NewInstrumentationScope()
	resource := pcommon.NewResource()
	cache := pcommon.NewMap()

	ctx := NewTransformContext(profile, instrumentationScope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles(), WithCache(&cache))

	logger := zap.NewExample()
	defer logger.Sync()

	logger.Info("test", zap.Object("context", ctx))

	/*	encoder := &mockObjectEncoder{fields: make(map[string]interface{})}
		err := ctx.MarshalLogObject(encoder)
		assert.NoError(t, err)

		assert.Contains(t, encoder.fields, "resource")
		assert.Contains(t, encoder.fields, "scope")
		assert.Contains(t, encoder.fields, "profile")
		assert.Contains(t, encoder.fields, "cache")
	*/
}
