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

package logs

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
	refLog, _, _ := createTelemetry()

	newAttrs := pcommon.NewMap()
	newAttrs.UpsertString("hello", "world")

	newArrStr := pcommon.NewValueSlice()
	newArrStr.SliceVal().AppendEmpty().SetStringVal("new")

	newArrBool := pcommon.NewValueSlice()
	newArrBool.SliceVal().AppendEmpty().SetBoolVal(false)

	newArrInt := pcommon.NewValueSlice()
	newArrInt.SliceVal().AppendEmpty().SetIntVal(20)

	newArrFloat := pcommon.NewValueSlice()
	newArrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)

	newArrBytes := pcommon.NewValueSlice()
	newArrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{9, 6, 4}))

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		new      interface{}
		modified func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource)
	}{
		{
			name: "time_unix_nano",
			path: []tql.Field{
				{
					Name: "time_unix_nano",
				},
			},
			orig: int64(100_000_000),
			new:  int64(200_000_000),
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
			orig: int64(500_000_000),
			new:  int64(200_000_000),
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
			orig: int64(plog.SeverityNumberFATAL),
			new:  int64(3),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSeverityNumber(plog.SeverityNumberTRACE3)
			},
		},
		{
			name: "severity_text",
			path: []tql.Field{
				{
					Name: "severity_text",
				},
			},
			orig: "blue screen of death",
			new:  "black screen of death",
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
			orig: "body",
			new:  "head",
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
			orig: int64(3),
			new:  int64(4),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetFlags(uint32(4))
			},
		},
		{
			name: "trace_id",
			path: []tql.Field{
				{
					Name: "trace_id",
				},
			},
			orig: pcommon.NewTraceID(traceID),
			new:  pcommon.NewTraceID(traceID2),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetTraceID(pcommon.NewTraceID(traceID2))
			},
		},
		{
			name: "span_id",
			path: []tql.Field{
				{
					Name: "span_id",
				},
			},
			orig: pcommon.NewSpanID(spanID),
			new:  pcommon.NewSpanID(spanID2),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSpanID(pcommon.NewSpanID(spanID2))
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
			orig: hex.EncodeToString(traceID[:]),
			new:  hex.EncodeToString(traceID2[:]),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetTraceID(pcommon.NewTraceID(traceID2))
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
			orig: hex.EncodeToString(spanID[:]),
			new:  hex.EncodeToString(spanID2[:]),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSpanID(pcommon.NewSpanID(spanID2))
			},
		},
		{
			name: "attributes",
			path: []tql.Field{
				{
					Name: "attributes",
				},
			},
			orig: refLog.Attributes(),
			new:  newAttrs,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Clear()
				newAttrs.CopyTo(log.Attributes())
			},
		},
		{
			name: "setting an attribute to nil is a no-op",
			path: []tql.Field{
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig: "val",
			new:  nil,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				// This behavior is undefined according to the spec, so it is implemented as a no-op.
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
			orig: "val",
			new:  "newVal",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().UpsertString("str", "newVal")
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
			orig: true,
			new:  false,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().UpsertBool("bool", false)
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
			orig: int64(10),
			new:  int64(20),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().UpsertInt("int", 20)
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
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().UpsertDouble("double", 2.4)
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
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
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
			new: []string{"new"},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Upsert("arr_str", newArrStr)
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
			new: []bool{false},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Upsert("arr_bool", newArrBool)
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
			new: []int64{20},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Upsert("arr_int", newArrInt)
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
			new: []float64{2.0},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Upsert("arr_float", newArrFloat)
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
			new: [][]byte{{9, 6, 4}},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
		{
			name: "dropped_attributes_count",
			path: []tql.Field{
				{
					Name: "dropped_attributes_count",
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetDroppedAttributesCount(20)
			},
		},
		{
			name: "instrumentation_scope name",
			path: []tql.Field{
				{
					Name: "instrumentation_scope",
				},
				{
					Name: "name",
				},
			},
			orig: "library",
			new:  "park",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				il.SetName("park")
			},
		},
		{
			name: "instrumentation_scope version",
			path: []tql.Field{
				{
					Name: "instrumentation_scope",
				},
				{
					Name: "version",
				},
			},
			orig: "version",
			new:  "next",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				il.SetVersion("next")
			},
		},
		{
			name: "resource attributes",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name: "attributes",
				},
			},
			orig: refLog.Attributes(),
			new:  newAttrs,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Clear()
				newAttrs.CopyTo(resource.Attributes())
			},
		},
		{
			name: "resource attributes string",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("str"),
				},
			},
			orig: "val",
			new:  "newVal",
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertString("str", "newVal")
			},
		},
		{
			name: "resource attributes bool",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bool"),
				},
			},
			orig: true,
			new:  false,
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertBool("bool", false)
			},
		},
		{
			name: "resource attributes int",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("int"),
				},
			},
			orig: int64(10),
			new:  int64(20),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertInt("int", 20)
			},
		},
		{
			name: "resource attributes float",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("double"),
				},
			},
			orig: float64(1.2),
			new:  float64(2.4),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertDouble("double", 2.4)
			},
		},
		{
			name: "resource attributes bytes",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("bytes"),
				},
			},
			orig: []byte{1, 3, 2},
			new:  []byte{2, 3, 4},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
			},
		},
		{
			name: "resource attributes array string",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_str"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_str")
				return val.SliceVal()
			}(),
			new: []string{"new"},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_str", newArrStr)
			},
		},
		{
			name: "resource attributes array bool",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bool"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_bool")
				return val.SliceVal()
			}(),
			new: []bool{false},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_bool", newArrBool)
			},
		},
		{
			name: "resource attributes array int",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_int"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_int")
				return val.SliceVal()
			}(),
			new: []int64{20},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_int", newArrInt)
			},
		},
		{
			name: "resource attributes array float",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_float"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_float")
				return val.SliceVal()
			}(),
			new: []float64{2.0},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_float", newArrFloat)
			},
		},
		{
			name: "resource attributes array bytes",
			path: []tql.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: tqltest.Strp("arr_bytes"),
				},
			},
			orig: func() pcommon.Slice {
				val, _ := refLog.Attributes().Get("arr_bytes")
				return val.SliceVal()
			}(),
			new: [][]byte{{9, 6, 4}},
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				resource.Attributes().Upsert("arr_bytes", newArrBytes)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := newPathGetSetter(tt.path)
			assert.NoError(t, err)

			log, il, resource := createTelemetry()

			got := accessor.Get(logTransformContext{
				log:      log,
				il:       il,
				resource: resource,
			})
			assert.Equal(t, tt.orig, got)

			accessor.Set(logTransformContext{
				log:      log,
				il:       il,
				resource: resource,
			}, tt.new)

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
	log.SetSeverityNumber(plog.SeverityNumberFATAL)
	log.SetSeverityText("blue screen of death")
	log.Body().SetStringVal("body")
	log.Attributes().UpsertString("str", "val")
	log.Attributes().UpsertBool("bool", true)
	log.Attributes().UpsertInt("int", 10)
	log.Attributes().UpsertDouble("double", 1.2)
	log.Attributes().UpsertBytes("bytes", pcommon.NewImmutableByteSlice([]byte{1, 3, 2}))

	arrStr := pcommon.NewValueSlice()
	arrStr.SliceVal().AppendEmpty().SetStringVal("one")
	arrStr.SliceVal().AppendEmpty().SetStringVal("two")
	log.Attributes().Upsert("arr_str", arrStr)

	arrBool := pcommon.NewValueSlice()
	arrBool.SliceVal().AppendEmpty().SetBoolVal(true)
	arrBool.SliceVal().AppendEmpty().SetBoolVal(false)
	log.Attributes().Upsert("arr_bool", arrBool)

	arrInt := pcommon.NewValueSlice()
	arrInt.SliceVal().AppendEmpty().SetIntVal(2)
	arrInt.SliceVal().AppendEmpty().SetIntVal(3)
	log.Attributes().Upsert("arr_int", arrInt)

	arrFloat := pcommon.NewValueSlice()
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(1.0)
	arrFloat.SliceVal().AppendEmpty().SetDoubleVal(2.0)
	log.Attributes().Upsert("arr_float", arrFloat)

	arrBytes := pcommon.NewValueSlice()
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{1, 2, 3}))
	arrBytes.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice([]byte{2, 3, 4}))
	log.Attributes().Upsert("arr_bytes", arrBytes)

	log.SetDroppedAttributesCount(10)

	log.SetFlags(uint32(3))

	log.SetTraceID(pcommon.NewTraceID(traceID))
	log.SetSpanID(pcommon.NewSpanID(spanID))

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
			want: 0,
		},
		{
			name: "SEVERITY_NUMBER_TRACE",
			want: 1,
		},
		{
			name: "SEVERITY_NUMBER_TRACE2",
			want: 2,
		},
		{
			name: "SEVERITY_NUMBER_TRACE3",
			want: 3,
		},
		{
			name: "SEVERITY_NUMBER_TRACE4",
			want: 4,
		},
		{
			name: "SEVERITY_NUMBER_DEBUG",
			want: 5,
		},
		{
			name: "SEVERITY_NUMBER_DEBUG2",
			want: 6,
		},
		{
			name: "SEVERITY_NUMBER_DEBUG3",
			want: 7,
		},
		{
			name: "SEVERITY_NUMBER_DEBUG4",
			want: 8,
		},
		{
			name: "SEVERITY_NUMBER_INFO",
			want: 9,
		},
		{
			name: "SEVERITY_NUMBER_INFO2",
			want: 10,
		},
		{
			name: "SEVERITY_NUMBER_INFO3",
			want: 11,
		},
		{
			name: "SEVERITY_NUMBER_INFO4",
			want: 12,
		},
		{
			name: "SEVERITY_NUMBER_WARN",
			want: 13,
		},
		{
			name: "SEVERITY_NUMBER_WARN2",
			want: 14,
		},
		{
			name: "SEVERITY_NUMBER_WARN3",
			want: 15,
		},
		{
			name: "SEVERITY_NUMBER_WARN4",
			want: 16,
		},
		{
			name: "SEVERITY_NUMBER_ERROR",
			want: 17,
		},
		{
			name: "SEVERITY_NUMBER_ERROR2",
			want: 18,
		},
		{
			name: "SEVERITY_NUMBER_ERROR3",
			want: 19,
		},
		{
			name: "SEVERITY_NUMBER_ERROR4",
			want: 20,
		},
		{
			name: "SEVERITY_NUMBER_FATAL",
			want: 21,
		},
		{
			name: "SEVERITY_NUMBER_FATAL2",
			want: 22,
		},
		{
			name: "SEVERITY_NUMBER_FATAL3",
			want: 23,
		},
		{
			name: "SEVERITY_NUMBER_FATAL4",

			want: 24,
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
