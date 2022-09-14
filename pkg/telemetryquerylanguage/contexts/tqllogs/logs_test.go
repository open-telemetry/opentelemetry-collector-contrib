// Copyright  The OpenTelemetry Authors
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
// See the License for the specific language governing permissions and
// limitations under the License.

package tqllogs

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

var (
	traceID  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	traceID2 = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	spanID   = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2  = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func Test_newPathGetSetter(t *testing.T) {
	refLog, refIS, refResource := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.PutString("hello", "world")

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource)
	}{
		{
			name: "time_unix_nano",
			path: []tql.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig:   int64(100_000_000),
			newVal: int64(200_000_000),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "observed_time_unix_nano",
			path: []tql.Field{
				{
					Name: "observed_time_unix_nano",
				},
			},
			orig:   int64(500_000_000),
			newVal: int64(200_000_000),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(200)))
			},
		},
		{
			name: "severity_number",
			path: []tql.Field{
				{
					Name: "severity_number",
				},
			},
			orig:   int64(plog.SeverityNumberFatal),
			newVal: int64(3),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSeverityNumber(plog.SeverityNumberTrace3)
			},
		},
		{
			name: "severity_text",
			path: []tql.Field{
				{
					Name: "severity_text",
				},
			},
			orig:   "blue screen of death",
			newVal: "black screen of death",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSeverityText("black screen of death")
			},
		},
		{
			name: "body",
			path: []tql.Field{
				{
					Name: "body",
				},
			},
			orig:   "body",
			newVal: "head",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Body().SetStringVal("head")
			},
		},
		{
			name: "flags",
			path: []tql.Field{
				{
					Name: "flags",
				},
			},
			orig:   int64(4),
			newVal: int64(5),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetFlags(plog.LogRecordFlags(5))
			},
		},
		{
			name: "trace_id",
			path: []tql.Field{
				{
					Name: "trace_id",
				},
			},
			orig:   pcommon.TraceID(traceID),
			newVal: pcommon.TraceID(traceID2),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id",
			path: []tql.Field{
				{
					Name: "span_id",
				},
			},
			orig:   pcommon.SpanID(spanID),
			newVal: pcommon.SpanID(spanID2),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSpanID(spanID2)
			},
		},
		{
			name: "trace_id string",
			path: []tql.Field{
				{
					Name: "trace_id",
				},
				{
					Name: "string",
				},
			},
			orig:   hex.EncodeToString(traceID[:]),
			newVal: hex.EncodeToString(traceID2[:]),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetTraceID(traceID2)
			},
		},
		{
			name: "span_id string",
			path: []tql.Field{
				{
					Name: "span_id",
				},
				{
					Name: "string",
				},
			},
			orig:   hex.EncodeToString(spanID[:]),
			newVal: hex.EncodeToString(spanID2[:]),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSpanID(spanID2)
			},
		},
		{
			name: "attributes",
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig:   refLog.Attributes(),
			newVal: newAttrs,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Clear()
				newAttrs.CopyTo(log.Attributes())
			},
		},
		{
			name: "attributes string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig:   "val",
			newVal: "newVal",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutString("str", "newVal")
			},
		},
		{
			name: "attributes bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
				},
			},
			orig:   true,
			newVal: false,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutBool("bool", false)
			},
		},
		{
			name: "attributes int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutInt("int", 20)
			},
		},
		{
			name: "attributes float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
				},
			},
			orig:   float64(1.2),
			newVal: float64(2.4),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutDouble("double", 2.4)
			},
		},
		{
			name: "attributes bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
				},
			},
			orig:   []byte{1, 3, 2},
			newVal: []byte{2, 3, 4},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmpty("bytes").SetEmptyBytesVal().FromRaw([]byte{2, 3, 4})
			},
		},
		{
			name: "attributes array string",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			newVal: []string{"new"},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmptySlice("arr_str").AppendEmpty().SetStringVal("new")
			},
		},
		{
			name: "attributes array bool",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			newVal: []bool{false},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmptySlice("arr_bool").AppendEmpty().SetBoolVal(false)
			},
		},
		{
			name: "attributes array int",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			newVal: []int64{20},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmptySlice("arr_int").AppendEmpty().SetIntVal(20)
			},
		},
		{
			name: "attributes array float",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			newVal: []float64{2.0},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmptySlice("arr_float").AppendEmpty().SetDoubleVal(2.0)
			},
		},
		{
			name: "attributes array bytes",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			newVal: [][]byte{{9, 6, 4}},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().PutEmptySlice("arr_bytes").AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{9, 6, 4})
			},
		},
		{
			name: "dropped_attributes_count",
			path: []tql.Field{
				{
					Name: "dropped_attributes_count",
				},
			},
			orig:   int64(10),
			newVal: int64(20),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "instrumentation_scope",
			path: []tql.Field{
				{
					Name: "instrumentation_scope",
				},
			},
			orig:   refIS,
			newVal: pcommon.NewInstrumentationScope(),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				pcommon.NewInstrumentationScope().CopyTo(il)
			},
		},
		{
			name: "resource",
			path: []tql.Field{
				{
					Name: "resource",
				},
			},
			orig:   refResource,
			newVal: pcommon.NewResource(),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				pcommon.NewResource().CopyTo(resource)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			log, il, resource := createTelemetry()

			got := accessor.Get(NewTransformContext(log, il, resource))
			assert.Equal(t, tt.orig, got)

			accessor.Set(NewTransformContext(log, il, resource), tt.newVal)

			exSpan, exIl, exRes := createTelemetry()
			tt.modified(exSpan, exIl, exRes)

			assert.Equal(t, exSpan, log)
			assert.Equal(t, exIl, il)
			assert.Equal(t, exRes, resource)
		})
	}
}

func createTelemetry() (plog.LogRecord, pcommon.InstrumentationScope, pcommon.Resource) {
	log := plog.NewLogRecord()
	log.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
	log.SetSeverityNumber(plog.SeverityNumberFatal)
	log.SetSeverityText("blue screen of death")
	log.Body().SetStringVal("body")
	log.Attributes().PutString("str", "val")
	log.Attributes().PutBool("bool", true)
	log.Attributes().PutInt("int", 10)
	log.Attributes().PutDouble("double", 1.2)
	log.Attributes().PutEmpty("bytes").SetEmptyBytesVal().FromRaw([]byte{1, 3, 2})

	arrStr := log.Attributes().PutEmptySlice("arr_str")
	arrStr.AppendEmpty().SetStringVal("one")
	arrStr.AppendEmpty().SetStringVal("two")

	arrBool := log.Attributes().PutEmptySlice("arr_bool")
	arrBool.AppendEmpty().SetBoolVal(true)
	arrBool.AppendEmpty().SetBoolVal(false)

	arrInt := log.Attributes().PutEmptySlice("arr_int")
	arrInt.AppendEmpty().SetIntVal(2)
	arrInt.AppendEmpty().SetIntVal(3)

	arrFloat := log.Attributes().PutEmptySlice("arr_float")
	arrFloat.AppendEmpty().SetDoubleVal(1.0)
	arrFloat.AppendEmpty().SetDoubleVal(2.0)

	arrBytes := log.Attributes().PutEmptySlice("arr_bytes")
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{1, 2, 3})
	arrBytes.AppendEmpty().SetEmptyBytesVal().FromRaw([]byte{2, 3, 4})

	log.SetDroppedAttributesCount(10)

	log.SetFlags(plog.LogRecordFlags(4))

	log.SetTraceID(traceID)
	log.SetSpanID(spanID)

	il := pcommon.NewInstrumentationScope()
	il.SetName("library")
	il.SetVersion("version")

	resource := pcommon.NewResource()
	log.Attributes().CopyTo(resource.Attributes())

	return log, il, resource
}

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want tql.Enum
	}{
		{
			name: "SEVERITY_NUMBER_UNSPECIFIED",
			want: tql.Enum(plog.SeverityNumberUndefined),
		},
		{
			name: "SEVERITY_NUMBER_TRACE",
			want: tql.Enum(plog.SeverityNumberTrace),
		},
		{
			name: "SEVERITY_NUMBER_TRACE2",
			want: tql.Enum(plog.SeverityNumberTrace2),
		},
		{
			name: "SEVERITY_NUMBER_TRACE3",
			want: tql.Enum(plog.SeverityNumberTrace3),
		},
		{
			name: "SEVERITY_NUMBER_TRACE4",
			want: tql.Enum(plog.SeverityNumberTrace4),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG",
			want: tql.Enum(plog.SeverityNumberDebug),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG2",
			want: tql.Enum(plog.SeverityNumberDebug2),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG3",
			want: tql.Enum(plog.SeverityNumberDebug3),
		},
		{
			name: "SEVERITY_NUMBER_DEBUG4",
			want: tql.Enum(plog.SeverityNumberDebug4),
		},
		{
			name: "SEVERITY_NUMBER_INFO",
			want: tql.Enum(plog.SeverityNumberInfo),
		},
		{
			name: "SEVERITY_NUMBER_INFO2",
			want: tql.Enum(plog.SeverityNumberInfo2),
		},
		{
			name: "SEVERITY_NUMBER_INFO3",
			want: tql.Enum(plog.SeverityNumberInfo3),
		},
		{
			name: "SEVERITY_NUMBER_INFO4",
			want: tql.Enum(plog.SeverityNumberInfo4),
		},
		{
			name: "SEVERITY_NUMBER_WARN",
			want: tql.Enum(plog.SeverityNumberWarn),
		},
		{
			name: "SEVERITY_NUMBER_WARN2",
			want: tql.Enum(plog.SeverityNumberWarn2),
		},
		{
			name: "SEVERITY_NUMBER_WARN3",
			want: tql.Enum(plog.SeverityNumberWarn3),
		},
		{
			name: "SEVERITY_NUMBER_WARN4",
			want: tql.Enum(plog.SeverityNumberWarn4),
		},
		{
			name: "SEVERITY_NUMBER_ERROR",
			want: tql.Enum(plog.SeverityNumberError),
		},
		{
			name: "SEVERITY_NUMBER_ERROR2",
			want: tql.Enum(plog.SeverityNumberError2),
		},
		{
			name: "SEVERITY_NUMBER_ERROR3",
			want: tql.Enum(plog.SeverityNumberError3),
		},
		{
			name: "SEVERITY_NUMBER_ERROR4",
			want: tql.Enum(plog.SeverityNumberError4),
		},
		{
			name: "SEVERITY_NUMBER_FATAL",
			want: tql.Enum(plog.SeverityNumberFatal),
		},
		{
			name: "SEVERITY_NUMBER_FATAL2",
			want: tql.Enum(plog.SeverityNumberFatal2),
		},
		{
			name: "SEVERITY_NUMBER_FATAL3",
			want: tql.Enum(plog.SeverityNumberFatal3),
		},
		{
			name: "SEVERITY_NUMBER_FATAL4",
			want: tql.Enum(plog.SeverityNumberFatal4),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseEnum((*tql.EnumSymbol)(tqltest.Strp(tt.name)))
			assert.NoError(t, err)
			assert.Equal(t, *actual, tt.want)
		})
	}
}

func Test_ParseEnum_False(t *testing.T) {
	tests := []struct {
		name       string
		enumSymbol *tql.EnumSymbol
	}{
		{
			name:       "unknown enum symbol",
			enumSymbol: (*tql.EnumSymbol)(tqltest.Strp("not an enum")),
		},
		{
			name:       "nil enum symbol",
			enumSymbol: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseEnum(tt.enumSymbol)
			assert.Error(t, err)
			assert.Nil(t, actual)
		})
	}
}
