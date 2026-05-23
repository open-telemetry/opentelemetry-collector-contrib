// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestTransformContextEncoder_AppendObject_NilObjectEncoderDoesNotPanic(t *testing.T) {
	enc := &transformContextEncoder{ObjectEncoder: nil, ArrayEncoder: nilObjectEncoderArrayEncoder{}}

	var called bool
	obj := zapcore.ObjectMarshalerFunc(func(zapcore.ObjectEncoder) error {
		called = true
		return nil
	})

	require.NoError(t, enc.AppendObject(obj))
	require.True(t, called)
}

func TestIsInvalidPData(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		invalid bool
	}{
		{"nil", nil, true},
		{"nil pointer to struct", (*objectMarshalerWithOrig)(nil), false},
		{"struct with nil orig", objectMarshalerWithOrig{}, true},
		{"struct with non-nil orig", objectMarshalerWithOrig{orig: new(int)}, false},
		{"non-struct string", "foo", false},
		{"non-struct int", 42, false},
		{"struct without orig field", struct{ X int }{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInvalidPData(tt.data)
			assert.Equal(t, tt.invalid, got)
		})
	}
}

func TestNewTransformContextField_NilProducesDeleted(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()

	m := &transformContextMarshaller{tCtx: nil}

	require.NoError(t, m.MarshalLogObject(enc))
	assert.Equal(t, deletedReplacement, enc.Fields[transformContextKey], "Fields[%q]", transformContextKey)
}

func TestNewTransformContextField_StructWithNilOrigProducesDeleted(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	m := &transformContextMarshaller{tCtx: objectMarshalerWithOrig{}}

	require.NoError(t, m.MarshalLogObject(enc))

	nested, ok := enc.Fields[transformContextKey].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, deletedReplacement, nested[deletedReplacement])
}

func TestTransformContextEncoder_AddObject_InvalidPDataProducesValueKey(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	wrapper := &transformContextEncoder{ObjectEncoder: enc}
	invalidObj := objectMarshalerWithOrig{}

	require.NoError(t, wrapper.AddObject("resource", invalidObj))

	nested, ok := enc.Fields["resource"].(map[string]any)
	require.True(t, ok, "Fields[\"resource\"] should be map[string]any")
	assert.Equal(t, deletedReplacement, nested[deletedReplacement])
}

func TestTransformContextEncoder_AddReflected_NilProducesDeleted(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	wrapper := &transformContextEncoder{ObjectEncoder: enc}

	require.NoError(t, wrapper.AddReflected("key", nil))

	assert.Equal(t, deletedReplacement, enc.Fields["key"])
}

func TestTransformContextEncoder_AddArray_InvalidPDataProducesDeletedElement(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	wrapper := &transformContextEncoder{ObjectEncoder: enc}
	invalidArr := arrayMarshalerWithOrig{}

	require.NoError(t, wrapper.AddArray("items", invalidArr))

	arr, ok := enc.Fields["items"].([]any)
	require.True(t, ok, "Fields[\"items\"] should be []any")
	require.Len(t, arr, 1)
	assert.Equal(t, deletedReplacement, arr[0])
}

func TestTransformContextEncoder_AppendArray_InvalidPDataProducesDeletedElement(t *testing.T) {
	var elements []any
	appendEnc := &sliceArrayEncoder{elems: &elements}
	wrapper := &transformContextEncoder{ObjectEncoder: nil, ArrayEncoder: appendEnc}
	invalidArr := arrayMarshalerWithOrig{}

	require.NoError(t, wrapper.AppendArray(invalidArr))

	require.Len(t, elements, 1)
	assert.Equal(t, deletedReplacement, elements[0])
}

func TestTransformContextEncoder_NestedObject_InvalidPDataChildProducesValueKey(t *testing.T) {
	enc := zapcore.NewMapObjectEncoder()
	wrapper := &transformContextEncoder{ObjectEncoder: enc}
	parent := nestedObjectMarshaler{}

	require.NoError(t, wrapper.AddObject("parent", parent))

	parentMap, ok := enc.Fields["parent"].(map[string]any)
	require.True(t, ok, `Fields["parent"] should be map[string]any`)

	childMap, ok := parentMap["child"].(map[string]any)
	require.True(t, ok, `Fields["parent"]["child"] should be map[string]any`)
	assert.Equal(t, deletedReplacement, childMap[deletedReplacement])
}

type objectMarshalerWithOrig struct{ orig *int }

func (objectMarshalerWithOrig) MarshalLogObject(zapcore.ObjectEncoder) error { return nil }

type arrayMarshalerWithOrig struct {
	//nolint:unused // used in isInvalidPData
	orig *int
}

func (arrayMarshalerWithOrig) MarshalLogArray(zapcore.ArrayEncoder) error { return nil }

type nestedObjectMarshaler struct{}

func (nestedObjectMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return enc.AddObject("child", objectMarshalerWithOrig{})
}

type sliceArrayEncoder struct{ elems *[]any }

func (s *sliceArrayEncoder) AppendArray(v zapcore.ArrayMarshaler) error {
	return v.MarshalLogArray(s)
}

func (s *sliceArrayEncoder) AppendObject(v zapcore.ObjectMarshaler) error {
	enc := zapcore.NewMapObjectEncoder()
	if err := v.MarshalLogObject(enc); err != nil {
		return err
	}
	*s.elems = append(*s.elems, enc.Fields)
	return nil
}

func (s *sliceArrayEncoder) AppendReflected(v any) error {
	*s.elems = append(*s.elems, v)
	return nil
}

func (s *sliceArrayEncoder) AppendString(v string)          { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendBool(v bool)              { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendByteString(v []byte)      { *s.elems = append(*s.elems, string(v)) }
func (s *sliceArrayEncoder) AppendComplex128(v complex128)  { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendComplex64(v complex64)    { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendDuration(v time.Duration) { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat64(v float64)        { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendFloat32(v float32)        { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendInt(v int)                { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendInt64(v int64)            { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendInt32(v int32)            { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendInt16(v int16)            { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendInt8(v int8)              { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendTime(v time.Time)         { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUint(v uint)              { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUint64(v uint64)          { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUint32(v uint32)          { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUint16(v uint16)          { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUint8(v uint8)            { *s.elems = append(*s.elems, v) }
func (s *sliceArrayEncoder) AppendUintptr(v uintptr)        { *s.elems = append(*s.elems, v) }

type nilObjectEncoderArrayEncoder struct{}

func (nilObjectEncoderArrayEncoder) AppendArray(v zapcore.ArrayMarshaler) error {
	return v.MarshalLogArray(nilObjectEncoderArrayEncoder{})
}

func (nilObjectEncoderArrayEncoder) AppendObject(v zapcore.ObjectMarshaler) error {
	return v.MarshalLogObject(nil)
}

func (nilObjectEncoderArrayEncoder) AppendReflected(_ any) error    { return nil }
func (nilObjectEncoderArrayEncoder) AppendBool(_ bool)              {}
func (nilObjectEncoderArrayEncoder) AppendByteString(_ []byte)      {}
func (nilObjectEncoderArrayEncoder) AppendComplex128(_ complex128)  {}
func (nilObjectEncoderArrayEncoder) AppendComplex64(_ complex64)    {}
func (nilObjectEncoderArrayEncoder) AppendDuration(_ time.Duration) {}
func (nilObjectEncoderArrayEncoder) AppendFloat64(_ float64)        {}
func (nilObjectEncoderArrayEncoder) AppendFloat32(_ float32)        {}
func (nilObjectEncoderArrayEncoder) AppendInt(_ int)                {}
func (nilObjectEncoderArrayEncoder) AppendInt64(_ int64)            {}
func (nilObjectEncoderArrayEncoder) AppendInt32(_ int32)            {}
func (nilObjectEncoderArrayEncoder) AppendInt16(_ int16)            {}
func (nilObjectEncoderArrayEncoder) AppendInt8(_ int8)              {}
func (nilObjectEncoderArrayEncoder) AppendString(_ string)          {}
func (nilObjectEncoderArrayEncoder) AppendTime(_ time.Time)         {}
func (nilObjectEncoderArrayEncoder) AppendUint(_ uint)              {}
func (nilObjectEncoderArrayEncoder) AppendUint64(_ uint64)          {}
func (nilObjectEncoderArrayEncoder) AppendUint32(_ uint32)          {}
func (nilObjectEncoderArrayEncoder) AppendUint16(_ uint16)          {}
func (nilObjectEncoderArrayEncoder) AppendUint8(_ uint8)            {}
func (nilObjectEncoderArrayEncoder) AppendUintptr(_ uintptr)        {}
