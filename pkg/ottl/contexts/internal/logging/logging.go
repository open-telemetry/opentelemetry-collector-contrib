// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logging // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

type Span ptrace.Span

func (s Span) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	ss := ptrace.Span(s)
	err := encoder.AddObject("attributes", Map(ss.Attributes()))
	encoder.AddUint32("dropped_attribute_count", ss.DroppedAttributesCount())
	encoder.AddUint32("dropped_events_count", ss.DroppedEventsCount())
	encoder.AddUint32("dropped_links_count", ss.DroppedLinksCount())
	encoder.AddUint64("end_time_unix_nano", uint64(ss.EndTimestamp()))
	err = errors.Join(err, encoder.AddArray("events", SpanEventSlice(ss.Events())))
	encoder.AddString("kind", ss.Kind().String())
	err = errors.Join(err, encoder.AddArray("links", SpanLinkSlice(ss.Links())))
	encoder.AddString("name", ss.Name())
	encoder.AddString("parent_span_id", ss.ParentSpanID().String())
	encoder.AddString("span_id", ss.SpanID().String())
	encoder.AddUint64("start_time_unix_nano", uint64(ss.StartTimestamp()))
	encoder.AddString("status.code", ss.Status().Code().String())
	encoder.AddString("status.message", ss.Status().Message())
	encoder.AddString("trace_id", ss.TraceID().String())
	encoder.AddString("trace_state", ss.TraceState().AsRaw())
	return err
}

type SpanEventSlice ptrace.SpanEventSlice

func (s SpanEventSlice) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	ses := ptrace.SpanEventSlice(s)
	var err error
	for i := 0; i < ses.Len(); i++ {
		se := ses.At(i)
		err = errors.Join(err, encoder.AppendObject(SpanEvent(se)))
	}
	return err
}

type SpanEvent ptrace.SpanEvent

func (s SpanEvent) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	se := ptrace.SpanEvent(s)
	err := encoder.AddObject("attributes", Map(se.Attributes()))
	encoder.AddUint32("dropped_attribute_count", se.DroppedAttributesCount())
	encoder.AddString("name", se.Name())
	encoder.AddUint64("time_unix_nano", uint64(se.Timestamp()))
	return err
}

type SpanLinkSlice ptrace.SpanLinkSlice

func (s SpanLinkSlice) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	sls := ptrace.SpanLinkSlice(s)
	var err error
	for i := 0; i < sls.Len(); i++ {
		sl := sls.At(i)
		err = errors.Join(err, encoder.AppendObject(SpanLink(sl)))
	}
	return err
}

type SpanLink ptrace.SpanLink

func (s SpanLink) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	sl := ptrace.SpanLink(s)
	err := encoder.AddObject("attributes", Map(sl.Attributes()))
	encoder.AddUint32("dropped_attribute_count", sl.DroppedAttributesCount())
	encoder.AddUint32("flags", sl.Flags())
	encoder.AddString("span_id", sl.SpanID().String())
	encoder.AddString("trace_id", sl.TraceID().String())
	encoder.AddString("trace_state", sl.TraceState().AsRaw())
	return err
}
