// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxlog_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

var (
	traceID1 = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID1  = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}

	time1 = time.Date(1970, 1, 1, 0, 0, 0, 100000000, time.UTC)
	time2 = time.Date(1970, 1, 1, 0, 0, 0, 200000000, time.UTC)
	time3 = time.Date(1970, 1, 1, 0, 0, 0, 300000000, time.UTC)
	time4 = time.Date(1970, 1, 1, 0, 0, 0, 400000000, time.UTC)
)

func TestPathGetSetter(t *testing.T) {
	refLog := createTelemetry("string")

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
		path     ottl.Path[*testContext]
		orig     any
		newVal   any
		modified func(log plog.LogRecord)
		bodyType string
	}{
		{
			name: "time_unix_nano",
			path: &pathtest.Path[*testContext]{
				N: "time_unix_nano",
			},
			orig:   time1.UnixNano(),
			newVal: time2.UnixNano(),
			modified: func(log plog.LogRecord) {
				log.SetTimestamp(pcommon.NewTimestampFromTime(time2))
			},
		},
		{
			name: "observed_time_unix_nano",
			path: &pathtest.Path[*testContext]{
				N: "observed_time_unix_nano",
			},
			orig:   time3.UnixNano(),
			newVal: time4.UnixNano(),
			modified: func(log plog.LogRecord) {
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time4))
			},
		},
		{
			name: "time",
			path: &pathtest.Path[*testContext]{
				N: "time",
			},
			orig:   time1,
			newVal: time2,
			modified: func(log plog.LogRecord) {
				log.SetTimestamp(pcommon.NewTimestampFromTime(time2))
			},
		},
		{
			name: "observed_time",
			path: &pathtest.Path[*testContext]{
				N: "observed_time",
			},
			orig:   time3,
			newVal: time4,
			modified: func(log plog.LogRecord) {
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time4))
			},
		},
		{
			name: "severity_number",
			path: &pathtest.Path[*testContext]{
				N: "severity_number",
			},
			orig:   int64(21),
			newVal: int64(9),
			modified: func(log plog.LogRecord) {
				log.SetSeverityNumber(plog.SeverityNumber(9))
			},
		},
		{
			name: "severity_text",
			path: &pathtest.Path[*testContext]{
				N: "severity_text",
			},
			orig:   "blue screen of death",
			newVal: "ERROR",
			modified: func(log plog.LogRecord) {
				log.SetSeverityText("ERROR")
			},
		},
		{
			name: "body is string",
			path: &pathtest.Path[*testContext]{
				N: "body",
			},
			orig:   "body",
			newVal: "new body",
			modified: func(log plog.LogRecord) {
				log.Body().SetStr("new body")
			},
			bodyType: "string",
		},
		{
			name: "body is int",
			path: &pathtest.Path[*testContext]{
				N: "body",
			},
			orig:   int64(1),
			newVal: int64(2),
			modified: func(log plog.LogRecord) {
				log.Body().SetInt(2)
			},
			bodyType: "int",
		},
		{
			name: "body is slice",
			path: &pathtest.Path[*testContext]{
				N: "body",
			},
			orig: func() pcommon.Slice {
				log := createTelemetry("slice")
				return log.Body().Slice()
			}(),
			newVal: newBodySlice,
			modified: func(log plog.LogRecord) {
				newBodySlice.CopyTo(log.Body().Slice())
			},
			bodyType: "slice",
		},
		{
			name: "body is map",
			path: &pathtest.Path[*testContext]{
				N: "body",
			},
			orig: func() pcommon.Map {
				log := createTelemetry("map")
				return log.Body().Map()
			}(),
			newVal: newBodyMap,
			modified: func(log plog.LogRecord) {
				newBodyMap.CopyTo(log.Body().Map())
			},
			bodyType: "map",
		},
		{
			name: "body.string",
			path: &pathtest.Path[*testContext]{
				N: "body",
				NextPath: &pathtest.Path[*testContext]{
					N: "string",
				},
			},
			orig:   "body",
			newVal: "new body",
			modified: func(log plog.LogRecord) {
				log.Body().SetStr("new body")
			},
			bodyType: "string",
		},
		{
			name: "body slice index",
			path: &pathtest.Path[*testContext]{
				N: "body",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						I: ottltest.Intp(0),
					},
				},
			},
			orig:   "body",
			newVal: "head",
			modified: func(log plog.LogRecord) {
				log.Body().Slice().At(0).SetStr("head")
			},
			bodyType: "slice",
		},
		{
			name: "body.map[key]",
			path: &pathtest.Path[*testContext]{
				N: "body",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("key"),
					},
				},
			},
			orig:   "val",
			newVal: "val2",
			modified: func(log plog.LogRecord) {
				log.Body().Map().PutStr("key", "val2")
			},
			bodyType: "map",
		},
		{
			name: "attributes",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:   refLog.Attributes(),
			newVal: newAttrs,
			modified: func(log plog.LogRecord) {
				newAttrs.CopyTo(log.Attributes())
			},
		},
		{
			name: "attributes raw map",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
			},
			orig:   refLog.Attributes(),
			newVal: newAttrs.AsRaw(),
			modified: func(log plog.LogRecord) {
				_ = log.Attributes().FromRaw(newAttrs.AsRaw())
			},
		},
		{
			name: "attributes.key",
			path: &pathtest.Path[*testContext]{
				N: "attributes",
				KeySlice: []ottl.Key[*testContext]{
					&pathtest.Key[*testContext]{
						S: ottltest.Strp("str"),
					},
				},
			},
			orig:   "val",
			newVal: "val2",
			modified: func(log plog.LogRecord) {
				log.Attributes().PutStr("str", "val2")
			},
		},
		{
			name: "dropped_attributes_count",
			path: &pathtest.Path[*testContext]{
				N: "dropped_attributes_count",
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(log plog.LogRecord) {
				log.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "flags",
			path: &pathtest.Path[*testContext]{
				N: "flags",
			},
			orig:   int64(4),
			newVal: int64(5),
			modified: func(log plog.LogRecord) {
				log.SetFlags(plog.LogRecordFlags(5))
			},
		},
		{
			name: "trace_id",
			path: &pathtest.Path[*testContext]{
				N: "trace_id",
			},
			orig:   pcommon.TraceID(traceID1),
			newVal: pcommon.TraceID(traceID2),
			modified: func(log plog.LogRecord) {
				log.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id",
			path: &pathtest.Path[*testContext]{
				N: "span_id",
			},
			orig:   pcommon.SpanID(spanID1),
			newVal: pcommon.SpanID(spanID2),
			modified: func(log plog.LogRecord) {
				log.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_id string",
			path: &pathtest.Path[*testContext]{
				N: "trace_id",
				NextPath: &pathtest.Path[*testContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(traceID1[:]),
			newVal: hex.EncodeToString(traceID2[:]),
			modified: func(log plog.LogRecord) {
				log.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id string",
			path: &pathtest.Path[*testContext]{
				N: "span_id",
				NextPath: &pathtest.Path[*testContext]{
					N: "string",
				},
			},
			orig:   hex.EncodeToString(spanID1[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(log plog.LogRecord) {
				log.SetSpanID(spanID2)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ctxlog.PathGetSetter(tt.path)
			assert.NoError(t, err)

			log := createTelemetry(tt.bodyType)

			got, err := accessor.Get(context.Background(), newTestContext(log))
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newTestContext(log), tt.newVal)
			assert.NoError(t, err)

			expectedLog := createTelemetry(tt.bodyType)
			tt.modified(expectedLog)

			assert.Equal(t, expectedLog, log)
		})
	}
}

func createTelemetry(bodyType string) plog.LogRecord {
	log := plog.NewLogRecord()
	log.SetTimestamp(pcommon.NewTimestampFromTime(time1))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time3))
	log.SetSeverityNumber(plog.SeverityNumberFatal)
	log.SetSeverityText("blue screen of death")
	log.Attributes().PutStr("str", "val")
	log.Attributes().PutBool("bool", true)
	log.Attributes().PutInt("int", 10)
	log.Attributes().PutDouble("double", 1.2)
	log.Attributes().PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})

	arrStr := log.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStr("one")
	arrStr.AppendEmpty().SetStr("two")

	arrBool := log.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBool(true)
	arrBool.AppendEmpty().SetBool(false)

	arrInt := log.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetInt(2)
	arrInt.AppendEmpty().SetInt(3)

	arrFloat := log.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDouble(1.0)
	arrFloat.AppendEmpty().SetDouble(2.0)

	arrBytes := log.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytes().FromRaw([]byte{2, 3, 4})

	pMap := log.Attributes().PutEmptyMap("pMap")
	pMap.PutStr("original", "map")

	m := log.Attributes().PutEmptyMap("map")
	m.PutStr("original", "map")

	s := log.Attributes().PutEmptySlice("slice")
	s.AppendEmpty().SetEmptyMap().PutStr("map", "pass")

	switch bodyType {
	case "map":
		log.Body().SetEmptyMap().PutStr("key", "val")
	case "slice":
		log.Body().SetEmptySlice().AppendEmpty().SetStr("body")
	case "int":
		log.Body().SetInt(1)
	default:
		log.Body().SetStr("body")
	}

	log.SetDroppedAttributesCount(10)

	log.SetFlags(plog.LogRecordFlags(4))

	log.SetTraceID(traceID1)
	log.SetSpanID(spanID1)

	return log
}

type testContext struct {
	log plog.LogRecord
}

func (l *testContext) GetLogRecord() plog.LogRecord {
	return l.log
}

func newTestContext(log plog.LogRecord) *testContext {
	return &testContext{log: log}
}
