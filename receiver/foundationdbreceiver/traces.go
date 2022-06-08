// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type SpanContext struct {
	TraceID [16]byte
	SpanID  uint64
}

type Event struct {
	Name       string
	EventTime  float64
	Attributes map[string]interface{}
}

type Trace struct {
	ArrLen         int
	TraceID        [16]byte
	SpanID         uint64
	ParentSpanID   uint64
	OperationName  string
	StartTimestamp float64
	EndTimestamp   float64
	Kind           uint8
	Status         uint8
	Links          []SpanContext
	Events         []Event
	Attributes     map[string]interface{}
}

func (t *Trace) DecodeMsgpack(dec *msgpack.Decoder) error {
	arrLen, err := dec.DecodeArrayLen()
	if err != nil {
		return err
	}

	traceFirst, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	traceSecond, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	traceId := getTraceId(traceFirst, traceSecond)

	spanID, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	parentSpanID, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	operation, err := dec.DecodeString()
	if err != nil {
		return err
	}

	startTime, err := dec.DecodeFloat64()
	if err != nil {
		return err
	}

	endTime, err := dec.DecodeFloat64()
	if err != nil {
		return err
	}

	kind, err := dec.DecodeUint8()
	if err != nil {
		return err
	}

	status, err := dec.DecodeUint8()
	if err != nil {
		return err
	}

	linksLen, err := dec.DecodeArrayLen()
	if err != nil {
		return err
	}

	var links []SpanContext
	for i := 0; i < linksLen; i++ {
		linkFirst, err := dec.DecodeUint64()
		if err != nil {
			return err
		}
		linkSecond, err := dec.DecodeUint64()
		if err != nil {
			return err
		}

		linkTraceId := getTraceId(linkFirst, linkSecond)
		spanId, err := dec.DecodeUint64()
		if err != nil {
			return err
		}
		links = append(links, SpanContext{TraceID: linkTraceId, SpanID: spanId})
	}

	eventsLen, err := dec.DecodeArrayLen()
	var events []Event
	for i := 0; i < eventsLen; i++ {
		eventName, err := dec.DecodeString()
		if err != nil {
			return err
		}
		eventTime, err := dec.DecodeFloat64()
		if err != nil {
			return err
		}
		attributes, err := dec.DecodeMap()
		if err != nil {
			return err
		}
		events = append(events, Event{Name: eventName, EventTime: eventTime, Attributes: attributes})
	}

	attributes, err := dec.DecodeMap()
	if err != nil {
		return err
	}

	t.ArrLen = arrLen
	t.TraceID = traceId
	t.SpanID = spanID
	t.ParentSpanID = parentSpanID
	t.OperationName = operation
	t.StartTimestamp = startTime
	t.EndTimestamp = endTime
	t.Status = status
	t.Kind = kind
	t.Links = links
	t.Events = events
	t.Attributes = attributes

	return nil
}

func (t *Trace) EncodeMsgpack(enc *msgpack.Encoder) error {
	err := enc.EncodeArrayLen(t.ArrLen)
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(binary.LittleEndian.Uint64(t.TraceID[0:8]))
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(binary.LittleEndian.Uint64(t.TraceID[8:16]))
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(t.SpanID)
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(t.ParentSpanID)
	if err != nil {
		return err
	}

	err = enc.EncodeString(t.OperationName)
	if err != nil {
		return err
	}

	err = enc.EncodeFloat64(t.StartTimestamp)
	if err != nil {
		return err
	}

	err = enc.EncodeFloat64(t.EndTimestamp)
	if err != nil {
		return err
	}

	err = enc.EncodeUint8(t.Kind)
	if err != nil {
		return err
	}

	err = enc.EncodeUint8(t.Status)
	if err != nil {
		return err
	}

	err = enc.EncodeArrayLen(len(t.Links))
	if err != nil {
		return err
	}
	for _, l := range t.Links {
		err = enc.EncodeUint64(binary.LittleEndian.Uint64(l.TraceID[0:8]))
		if err != nil {
			return err
		}
		err = enc.EncodeUint64(binary.LittleEndian.Uint64(l.TraceID[8:16]))
		if err != nil {
			return err
		}
		err = enc.EncodeUint64(l.SpanID)
		if err != nil {
			return err
		}
	}

	err = enc.EncodeArrayLen(len(t.Events))
	if err != nil {
		return err
	}
	for _, e := range t.Events {
		err = enc.EncodeString(e.Name)
		if err != nil {
			return err
		}
		err = enc.EncodeFloat64(e.EventTime)
		if err != nil {
			return err
		}
		err = enc.EncodeMap(e.Attributes)
		if err != nil {
			return err
		}
	}

	err = enc.EncodeMap(t.Attributes)
	return err
}

func isPrintable(b byte) bool {
	return 32 <= b && b <= 126
}

func base16Char(c byte) (error, byte) {
	switch (c%16 + 16) % 16 {
	case 0:
		return nil, '0'
	case 1:
		return nil, '1'
	case 2:
		return nil, '2'
	case 3:
		return nil, '3'
	case 4:
		return nil, '4'
	case 5:
		return nil, '5'
	case 6:
		return nil, '6'
	case 7:
		return nil, '7'
	case 8:
		return nil, '8'
	case 9:
		return nil, '9'
	case 10:
		return nil, 'a'
	case 11:
		return nil, 'b'
	case 12:
		return nil, 'c'
	case 13:
		return nil, 'd'
	case 14:
		return nil, 'e'
	case 15:
		return nil, 'f'
	default:
		return fmt.Errorf("unformattable byte"), 0
	}
}

func formatTag(v string) string {
	var result []byte
	data := []byte(v)
	for _, c := range data {
		if c == '\\' {
			result = append(result, '\\')
			result = append(result, '\\')
		} else if isPrintable(c) {
			result = append(result, c)
		} else {
			result = append(result, '\\')
			result = append(result, 'x')
			err, b := base16Char(c / 16)
			if err == nil {
				result = append(result, b)
			}
			err, b = base16Char(c)
			if err == nil {
				result = append(result, b)
			}
		}
	}
	return string(result)
}

func (t *Trace) getSpan(span *ptrace.Span) {
	span.SetTraceID(pcommon.NewTraceID(t.TraceID))
	span.SetSpanID(pcommon.NewSpanID(uint64ToBytes(t.SpanID)))
	span.SetParentSpanID(pcommon.NewSpanID(uint64ToBytes(t.ParentSpanID)))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(timestampFromFloat64(t.StartTimestamp)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(timestampFromFloat64(t.EndTimestamp)))
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetName(t.OperationName)
	span.SetDroppedEventsCount(0)
	span.SetDroppedAttributesCount(0)
	span.SetDroppedLinksCount(0)

	attrs := span.Attributes()
	for k, v := range t.Attributes {
		formattedKey := formatTag(k)
		formattedValue := formatTag(v.(string))
		attrs.InsertString(formattedKey, formattedValue)
	}

	links := span.Links()
	for _, l := range t.Links {
		link := links.AppendEmpty()
		link.SetTraceID(pcommon.NewTraceID(l.TraceID))
		link.SetSpanID(pcommon.NewSpanID(uint64ToBytes(l.SpanID)))
	}

	events := span.Events()
	for _, e := range t.Events {
		event := events.AppendEmpty()
		event.SetName(e.Name)
		event.SetTimestamp(pcommon.NewTimestampFromTime(timestampFromFloat64(e.EventTime)))
		attrs := event.Attributes()
		for k, v := range e.Attributes {
			attrs.InsertString(k, v.(string))
		}
	}
}

type OpenTracing struct {
	ArrLen         int
	SourceIP       string
	TraceID        uint64
	SpanID         uint64
	StartTimestamp float64
	Duration       float64
	OperationName  string
	Tags           map[string]interface{}
	ParentSpanIDs  []interface{}
}

var _ msgpack.CustomDecoder = (*OpenTracing)(nil)

func (t *OpenTracing) DecodeMsgpack(dec *msgpack.Decoder) error {
	arrLen, err := dec.DecodeArrayLen()
	if err != nil {
		return err
	}

	sourceIP, err := dec.DecodeString()
	if err != nil {
		return err
	}

	traceID, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	spanID, err := dec.DecodeUint64()
	if err != nil {
		return err
	}

	startTimestamp, err := dec.DecodeFloat64()
	if err != nil {
		return err
	}

	duration, err := dec.DecodeFloat64()
	if err != nil {
		return err
	}

	operation, err := dec.DecodeString()
	if err != nil {
		return err
	}

	tags, err := dec.DecodeMap()
	if err != nil {
		return err
	}

	parentIds, err := dec.DecodeSlice()
	if err != nil {
		return err
	}

	t.ArrLen = arrLen
	t.SourceIP = sourceIP
	t.TraceID = traceID
	t.SpanID = spanID
	t.StartTimestamp = startTimestamp
	t.Duration = duration
	t.OperationName = operation
	t.Tags = tags
	t.ParentSpanIDs = parentIds

	return nil
}

func (t *OpenTracing) EncodeMsgpack(enc *msgpack.Encoder) error {
	err := enc.EncodeArrayLen(t.ArrLen)
	if err != nil {
		return err
	}

	err = enc.EncodeString(t.SourceIP)
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(t.TraceID)
	if err != nil {
		return err
	}

	err = enc.EncodeUint64(t.SpanID)
	if err != nil {
		return err
	}

	err = enc.EncodeFloat64(t.StartTimestamp)
	if err != nil {
		return err
	}

	err = enc.EncodeFloat64(t.Duration)
	if err != nil {
		return err
	}

	err = enc.EncodeString(t.OperationName)
	if err != nil {
		return err
	}

	err = enc.EncodeMap(t.Tags)
	if err != nil {
		return err
	}

	err = enc.EncodeMulti(t.ParentSpanIDs)
	return err
}

func (t *OpenTracing) getSpan(span *ptrace.Span) {
	span.SetTraceID(pcommon.NewTraceID(uint64ToOtelTraceId(t.TraceID)))
	span.SetSpanID(pcommon.NewSpanID(uint64ToBytes(t.SpanID)))
	endTime := timestampFromFloat64(t.StartTimestamp)
	durSec, durNano := durationFromFloat64(t.Duration)
	endTime = endTime.Add(time.Second * time.Duration(durSec))
	endTime = endTime.Add(time.Nanosecond * time.Duration(durNano))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(timestampFromFloat64(t.StartTimestamp)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
	span.SetKind(ptrace.SpanKindServer)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetName(t.OperationName)
	span.SetDroppedEventsCount(0)
	span.SetDroppedAttributesCount(0)
	span.SetDroppedLinksCount(0)
	if len(t.ParentSpanIDs) > 0 {
		pId := t.ParentSpanIDs[0].(uint64)
		span.SetParentSpanID(pcommon.NewSpanID(uint64ToBytes(pId)))
	}

	attrs := span.Attributes()
	attrs.InsertString("sourceIP", t.SourceIP)
	for k, v := range t.Tags {
		attrs.InsertString(k, v.(string))
	}
}

func getTraceId(first, second uint64) [16]byte {
	firstTrace := uint64ToBytes(first)
	secondTrace := uint64ToBytes(second)
	var traceId [16]byte
	for i := 0; i < 16; i++ {
		if i < 8 {
			traceId[i] = firstTrace[i]
		} else {
			traceId[i] = secondTrace[i-8]
		}
	}
	return traceId
}

func durationFromFloat64(ts float64) (int64, int64) {
	secs := int64(ts)
	nsecs := int64((ts - float64(secs)) * 1e9)
	return secs, nsecs
}

func timestampFromFloat64(ts float64) time.Time {
	secs, nsecs := durationFromFloat64(ts)
	t := time.Unix(secs, nsecs)
	return t
}

func uint64ToBytes(v uint64) [8]byte {
	b := [8]byte{
		byte(0xff & v),
		byte(0xff & (v >> 8)),
		byte(0xff & (v >> 16)),
		byte(0xff & (v >> 24)),
		byte(0xff & (v >> 32)),
		byte(0xff & (v >> 40)),
		byte(0xff & (v >> 48)),
		byte(0xff & (v >> 56))}
	return b
}

func uint64ToOtelTraceId(traceID uint64) [16]byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, traceID)
	var td [16]byte
	for i, x := range b {
		td[i+8] = x
	}
	return td
}
