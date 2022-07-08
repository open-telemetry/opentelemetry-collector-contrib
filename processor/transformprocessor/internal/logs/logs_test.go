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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
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
		path     []common.Field
		orig     interface{}
		new      interface{}
		modified func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource)
	}{
		{
			name: "time_unix_nano",
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
				{
					Name: "severity_number",
				},
			},
			orig: plog.SeverityNumberFATAL,
			new:  int64(3),
			modified: func(log plog.LogRecord, il pcommon.InstrumentationScope, resource pcommon.Resource) {
				log.SetSeverityNumber(plog.SeverityNumberTRACE3)
			},
		},
		{
			name: "severity_text",
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
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
			name: "attributes",
			path: []common.Field{
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
			name: "attributes string",
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("str"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bool"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("int"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("double"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bytes"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_str"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bool"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_int"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_float"),
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
			path: []common.Field{
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bytes"),
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
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("str"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bool"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("int"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("double"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("bytes"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_str"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bool"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_int"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_float"),
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
			path: []common.Field{
				{
					Name: "resource",
				},
				{
					Name:   "attributes",
					MapKey: testhelper.Strp("arr_bytes"),
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
