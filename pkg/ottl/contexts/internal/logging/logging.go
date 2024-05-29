// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logging // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap/zapcore"
)

type Slice pcommon.Slice

func (s Slice) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	ss := pcommon.Slice(s)
	var err error
	for i := 0; i < ss.Len(); i++ {
		v := ss.At(i)
		switch v.Type() {
		case pcommon.ValueTypeStr:
			encoder.AppendString(v.Str())
		case pcommon.ValueTypeBool:
			encoder.AppendBool(v.Bool())
		case pcommon.ValueTypeInt:
			encoder.AppendInt64(v.Int())
		case pcommon.ValueTypeDouble:
			encoder.AppendFloat64(v.Double())
		case pcommon.ValueTypeMap:
			err = errors.Join(err, encoder.AppendObject(Map(v.Map())))
		case pcommon.ValueTypeSlice:
			err = errors.Join(err, encoder.AppendArray(Slice(v.Slice())))
		case pcommon.ValueTypeBytes:
			encoder.AppendByteString(v.Bytes().AsRaw())
		}
	}
	return err
}

type Map pcommon.Map

func (m Map) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	mm := pcommon.Map(m)
	var err error
	mm.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			encoder.AddString(k, v.Str())
		case pcommon.ValueTypeBool:
			encoder.AddBool(k, v.Bool())
		case pcommon.ValueTypeInt:
			encoder.AddInt64(k, v.Int())
		case pcommon.ValueTypeDouble:
			encoder.AddFloat64(k, v.Double())
		case pcommon.ValueTypeMap:
			err = errors.Join(err, encoder.AddObject(k, Map(v.Map())))
		case pcommon.ValueTypeSlice:
			err = errors.Join(err, encoder.AddArray(k, Slice(v.Slice())))
		case pcommon.ValueTypeBytes:
			encoder.AddByteString(k, v.Bytes().AsRaw())
		}
		return true
	})
	return nil
}

type Resource pcommon.Resource

func (r Resource) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	rr := pcommon.Resource(r)
	err := encoder.AddObject("attributes", Map(rr.Attributes()))
	encoder.AddUint32("dropped_attribute_count", rr.DroppedAttributesCount())
	return err
}

type InstrumentationScope pcommon.InstrumentationScope

func (i InstrumentationScope) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	is := pcommon.InstrumentationScope(i)
	err := encoder.AddObject("attributes", Map(is.Attributes()))
	encoder.AddUint32("dropped_attribute_count", is.DroppedAttributesCount())
	encoder.AddString("name", is.Name())
	encoder.AddString("version", is.Version())
	return err
}
