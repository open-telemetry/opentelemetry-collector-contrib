// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func TestValueTypeGetterSetter(t *testing.T) {
	tests := []struct {
		name      string
		path      ottl.Path[*mockValueTypeContext]
		wantErr   bool
		checkFunc func(t *testing.T, getSetter any, err error)
	}{
		{
			name:    "nil path",
			path:    nil,
			wantErr: false,
			checkFunc: func(t *testing.T, getSetter any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, getSetter)

				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()

				gs, ok := getSetter.(ottl.GetSetter[*mockValueTypeContext])
				assert.True(t, ok)

				ctx := &mockValueTypeContext{dictionary: dict, valueType: valueType}
				got, err := gs.Get(t.Context(), ctx)
				require.NoError(t, err)
				assert.Equal(t, valueType, got)
			},
		},
		{
			name: "type path",
			path: &pathtest.Path[*mockValueTypeContext]{
				N: "type",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, getSetter any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, getSetter)
			},
		},
		{
			name: "unit path",
			path: &pathtest.Path[*mockValueTypeContext]{
				N: "unit",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, getSetter any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, getSetter)
			},
		},
		{
			name: "invalid path",
			path: &pathtest.Path[*mockValueTypeContext]{
				N:        "sample_type",
				NextPath: &pathtest.Path[*mockValueTypeContext]{N: "invalid_field"},
			},
			wantErr: true,
			checkFunc: func(t *testing.T, getSetter any, err error) {
				assert.Error(t, err)
				assert.Nil(t, getSetter)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getSetter, err := valueTypeGetterSetter(tt.path, mockTargetValueType)
			tt.checkFunc(t, getSetter, err)
		})
	}
}

func TestAccessValueTypeRoot_Getter(t *testing.T) {
	dict := newEmptyProfilesDictionary()

	// Set up string table
	strTable := dict.StringTable()
	strTable.Append("cpu")
	strTable.Append("nanoseconds")

	valueType := pprofile.NewValueType()
	valueType.SetTypeStrindex(1)
	valueType.SetUnitStrindex(2)

	ctx := &mockValueTypeContext{
		dictionary: dict,
		valueType:  valueType,
	}

	getSetter := accessValueType(&pathtest.Path[*mockValueTypeContext]{N: "period_type"}, mockTargetValueType)
	got, err := getSetter.Get(t.Context(), ctx)
	require.NoError(t, err)

	gotValueType, ok := got.(pprofile.ValueType)
	assert.True(t, ok)
	assert.Equal(t, int32(1), gotValueType.TypeStrindex())
	assert.Equal(t, int32(2), gotValueType.UnitStrindex())
}

func TestAccessValueTypeRoot_Setter(t *testing.T) {
	tests := []struct {
		name     string
		setValue any
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid value",
			setValue: func() pprofile.ValueType {
				vt := pprofile.NewValueType()
				vt.SetTypeStrindex(2)
				vt.SetUnitStrindex(1)
				return vt
			}(),
		},
		{
			name:     "invalid value",
			setValue: "not_a_value_type",
			wantErr:  true,
			errMsg:   "path \"sample_type\" expects pprofile.ValueType but got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := newEmptyProfilesDictionary()
			originalStringTable := pcommon.NewStringSlice()
			dict.StringTable().CopyTo(originalStringTable)

			valueType := pprofile.NewValueType()
			ctx := &mockValueTypeContext{dictionary: dict, valueType: valueType}

			getSetter := accessValueType(&pathtest.Path[*mockValueTypeContext]{N: "sample_type"}, mockTargetValueType)
			err := getSetter.Set(t.Context(), ctx, tt.setValue)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, originalStringTable, dict.StringTable())
			}
		})
	}
}

func TestAccessValueTypeType_Getter(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *mockValueTypeContext
		wantValue string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				strTable := dict.StringTable()
				strTable.Append("cpu")
				strTable.Append("memory")
				valueType := pprofile.NewValueType()
				valueType.SetTypeStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			wantValue: "cpu",
		},
		{
			name: "out of range",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				valueType.SetTypeStrindex(10)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			wantErr: true,
			errMsg:  "path \"type\" with strindex 10 is out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupFunc()
			getSetter := accessValueTypeType(&pathtest.Path[*mockValueTypeContext]{N: "type"}, mockTargetValueType)
			got, err := getSetter.Get(t.Context(), ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, got)
			}
		})
	}
}

func TestAccessValueTypeType_Setter(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() *mockValueTypeContext
		setValue     any
		wantErr      bool
		errMsg       string
		validateFunc func(t *testing.T, ctx *mockValueTypeContext, originalIdx int32)
	}{
		{
			name: "new value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				valueType.SetTypeStrindex(0)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "cpu",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, _ int32) {
				typeIdx := ctx.valueType.TypeStrindex()
				strTable := ctx.dictionary.StringTable()
				assert.Positive(t, typeIdx)
				assert.Equal(t, "cpu", strTable.At(int(typeIdx)))
			},
		},
		{
			name: "update same value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				dict.StringTable().Append("cpu")
				valueType := pprofile.NewValueType()
				valueType.SetTypeStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "cpu",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, originalIdx int32) {
				assert.Equal(t, originalIdx, ctx.valueType.TypeStrindex())
			},
		},
		{
			name: "update different value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				dict.StringTable().Append("cpu")
				valueType := pprofile.NewValueType()
				valueType.SetTypeStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "memory",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, _ int32) {
				typeIdx := ctx.valueType.TypeStrindex()
				strTable := ctx.dictionary.StringTable()
				assert.Greater(t, typeIdx, int32(1))
				assert.Equal(t, "memory", strTable.At(int(typeIdx)))
			},
		},
		{
			name: "invalid type",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: 123,
			wantErr:  true,
			errMsg:   "path \"type\" expects string but got int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupFunc()
			originalStringTable := pcommon.NewStringSlice()
			ctx.dictionary.StringTable().CopyTo(originalStringTable)
			originalIdx := ctx.valueType.TypeStrindex()

			getSetter := accessValueTypeType(&pathtest.Path[*mockValueTypeContext]{N: "type"}, mockTargetValueType)
			err := getSetter.Set(t.Context(), ctx, tt.setValue)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, ctx, originalIdx)
				}
				for idx, val := range originalStringTable.All() {
					originalVal := ctx.dictionary.StringTable().At(idx)
					assert.Equal(t, val, originalVal)
				}
			}
		})
	}
}

func TestAccessValueTypeUnit_Getter(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *mockValueTypeContext
		wantValue string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				strTable := dict.StringTable()
				strTable.Append("nanoseconds")
				strTable.Append("bytes")
				valueType := pprofile.NewValueType()
				valueType.SetUnitStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			wantValue: "nanoseconds",
			wantErr:   false,
		},
		{
			name: "out of range",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				valueType.SetUnitStrindex(10)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			wantErr: true,
			errMsg:  "path \"unit\" with strindex 10 is out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupFunc()
			getSetter := accessValueTypeUnit(&pathtest.Path[*mockValueTypeContext]{N: "unit"}, mockTargetValueType)
			got, err := getSetter.Get(t.Context(), ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, got)
			}
		})
	}
}

func TestAccessValueTypeUnit_Setter(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() *mockValueTypeContext
		setValue     any
		wantErr      bool
		errMsg       string
		validateFunc func(t *testing.T, ctx *mockValueTypeContext, originalIdx int32)
	}{
		{
			name: "new value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				valueType.SetUnitStrindex(0)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "nanoseconds",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, _ int32) {
				unitIdx := ctx.valueType.UnitStrindex()
				strTable := ctx.dictionary.StringTable()
				assert.Positive(t, unitIdx)
				assert.Equal(t, "nanoseconds", strTable.At(int(unitIdx)))
			},
		},
		{
			name: "update same value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				strTable := dict.StringTable()
				strTable.Append("nanoseconds")
				valueType := pprofile.NewValueType()
				valueType.SetUnitStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "nanoseconds",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, originalIdx int32) {
				assert.Equal(t, originalIdx, ctx.valueType.UnitStrindex())
			},
		},
		{
			name: "update different value",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				dict.StringTable().Append("nanoseconds")
				valueType := pprofile.NewValueType()
				valueType.SetUnitStrindex(1)
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: "bytes",
			wantErr:  false,
			validateFunc: func(t *testing.T, ctx *mockValueTypeContext, _ int32) {
				unitIdx := ctx.valueType.UnitStrindex()
				strTable := ctx.dictionary.StringTable()
				assert.Greater(t, unitIdx, int32(1))
				assert.Equal(t, "bytes", strTable.At(int(unitIdx)))
			},
		},
		{
			name: "invalid type",
			setupFunc: func() *mockValueTypeContext {
				dict := newEmptyProfilesDictionary()
				valueType := pprofile.NewValueType()
				return &mockValueTypeContext{dictionary: dict, valueType: valueType}
			},
			setValue: 456,
			wantErr:  true,
			errMsg:   "path \"unit\" expects string but got int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupFunc()
			originalStringTable := pcommon.NewStringSlice()
			ctx.dictionary.StringTable().CopyTo(originalStringTable)
			originalIdx := ctx.valueType.UnitStrindex()

			getSetter := accessValueTypeUnit(&pathtest.Path[*mockValueTypeContext]{N: "unit"}, mockTargetValueType)
			err := getSetter.Set(t.Context(), ctx, tt.setValue)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
			} else {
				require.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, ctx, originalIdx)
				}
				for idx, val := range originalStringTable.All() {
					originalVal := ctx.dictionary.StringTable().At(idx)
					assert.Equal(t, val, originalVal)
				}
			}
		})
	}
}

func newEmptyProfilesDictionary() pprofile.ProfilesDictionary {
	dict := pprofile.NewProfilesDictionary()
	// table index 0 is always empty string
	dict.StringTable().Append("")
	return dict
}

type mockValueTypeContext struct {
	profile    pprofile.Profile
	dictionary pprofile.ProfilesDictionary
	valueType  pprofile.ValueType
}

func (m *mockValueTypeContext) GetProfile() pprofile.Profile {
	return m.profile
}

func (m *mockValueTypeContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return m.dictionary
}

func mockTargetValueType(ctx *mockValueTypeContext) pprofile.ValueType {
	return ctx.valueType
}
