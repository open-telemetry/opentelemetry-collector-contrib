// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestCreateTracesExporter(t *testing.T) {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			logs, err := createTracesExporter(ctx, createSettings, tt.config)

			if err == nil {
				assert.Nil(t, tt.expectedError)
				assert.NotNil(t, logs)
			} else {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, logs)
			}
		})
	}
}

func generateTEvent1Raw() *add_events.Event {
	return &add_events.Event{
		Thread: "TT",
		Log:    "LT",
		Sev:    9,
		Ts:     "1581452772000000321",
		Attrs: map[string]interface{}{
			"sca:schemVer": 1,
			"sca:schema":   "tracing",
			"sca:type":     "span",

			"name": "operationA",
			"kind": "unspecified",

			"start_time_unix_nano": "1581452772000000321",
			"end_time_unix_nano":   "1581452773000000789",
			"duration_nano":        "1000000468",

			"span_id":        "",
			"trace_id":       "",
			"resource_name":  "",
			"resource_type":  "process",
			"status_code":    "error",
			"status_message": "status-cancelled",
		},
	}
}

func generateTEvent2Raw() *add_events.Event {
	return &add_events.Event{
		Thread: "TT",
		Log:    "LT",
		Sev:    9,
		Ts:     "1581452772000000321",
		Attrs: map[string]interface{}{
			"sca:schemVer": 1,
			"sca:schema":   "tracing",
			"sca:type":     "span",

			"name": "operationB",
			"kind": "unspecified",

			"start_time_unix_nano": "1581452772000000321",
			"end_time_unix_nano":   "1581452773000000789",
			"duration_nano":        "1000000468",

			"span_id":        "",
			"trace_id":       "",
			"status_code":    "unset",
			"status_message": "",
			"resource_name":  "",
			"resource_type":  "process",
		},
	}
}

func generateTEvent3Raw() *add_events.Event {
	return &add_events.Event{
		Thread: "TT",
		Log:    "LT",
		Sev:    9,
		Ts:     "1581452772000000321",
		Attrs: map[string]interface{}{
			"sca:schemVer": 1,
			"sca:schema":   "tracing",
			"sca:type":     "span",

			"name": "operationC",
			"kind": "unspecified",

			"start_time_unix_nano": "1581452772000000321",
			"end_time_unix_nano":   "1581452773000000789",
			"duration_nano":        "1000000468",

			"span_id":        "",
			"trace_id":       "",
			"span-attr":      "span-attr-val",
			"status_code":    "unset",
			"status_message": "",
			"resource_name":  "",
			"resource_type":  "process",
		},
	}
}

var testTThread = &add_events.Thread{
	Id:   "TT",
	Name: "traces",
}

var testTLog = &add_events.Log{
	Id:    "LT",
	Attrs: map[string]interface{}{},
}

func TestBuildEventFromSpanOne(t *testing.T) {
	traces := testdata.GenerateTracesOneSpan()
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	expected := &add_events.EventBundle{
		Event:  generateTEvent1Raw(),
		Thread: testTThread,
		Log:    testTLog,
	}
	was := buildEventFromSpan(
		spanBundle{
			span,
			traces.ResourceSpans().At(0).Resource(),
			traces.ResourceSpans().At(0).ScopeSpans().At(0).Scope(),
		},
	)

	assert.Equal(t, expected, was)
}

func TestBuildEventsFromSpanAttributesCollision(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rss := rs.ScopeSpans().AppendEmpty()
	span := rss.Spans().AppendEmpty()
	span.Attributes().PutStr("name", "should_be_name_")
	span.Attributes().PutStr("span_id", "should_be_span_id_")
	expected := &add_events.EventBundle{
		Event: &add_events.Event{
			Thread: "TT",
			Log:    "LT",
			Sev:    9,
			Ts:     "0",
			Attrs: map[string]interface{}{
				"sca:schemVer": 1,
				"sca:schema":   "tracing",
				"sca:type":     "span",

				"name": "",
				"kind": "unspecified",

				"start_time_unix_nano": "0",
				"end_time_unix_nano":   "0",
				"duration_nano":        "0",

				"span_id":        "",
				"trace_id":       "",
				"status_code":    "unset",
				"status_message": "",
				"resource_name":  "",
				"resource_type":  "process",
				"name_":          "should_be_name_",
				"span_id_":       "should_be_span_id_",
			},
		},
		Thread: testTThread,
		Log:    testTLog,
	}
	was := buildEventFromSpan(
		spanBundle{
			span,
			rs.Resource(),
			rss.Scope(),
		},
	)

	assert.Equal(t, expected, was)
}

func TestBuildEventsFromTracesFromTwoSpansSameResourceOneDifferent(t *testing.T) {
	traces := testdata.GenerateTracesTwoSpansSameResourceOneDifferent()
	was := buildEventsFromTraces(traces)

	expected := []*add_events.EventBundle{
		{
			Event:  generateTEvent1Raw(),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateTEvent2Raw(),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateTEvent3Raw(),
			Thread: testTThread,
			Log:    testTLog,
		},
	}

	assert.Equal(t, expected, was)
}

var span0Id = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
var span00Id = [8]byte{1, 2, 1, 1, 1, 1, 1, 1}
var span01Id = [8]byte{1, 3, 1, 1, 1, 1, 1, 1}
var span000Id = [8]byte{1, 2, 2, 1, 1, 1, 1, 1}
var span001Id = [8]byte{1, 2, 3, 1, 1, 1, 1, 1}
var span002Id = [8]byte{1, 2, 4, 1, 1, 1, 1, 1}

var span1Id = [8]byte{2, 2, 2, 2, 2, 2, 2, 2}
var span10Id = [8]byte{2, 3, 2, 2, 2, 2, 2, 2}

var span21Id = [8]byte{3, 3, 3, 3, 3, 3, 3, 3}
var span22Id = [8]byte{3, 4, 3, 3, 3, 3, 3, 3}

var span21PId = [8]byte{3, 5, 3, 3, 3, 3, 3, 3}
var span22PId = [8]byte{3, 6, 3, 3, 3, 3, 3, 3}

var span3Id = [8]byte{4, 4, 4, 4, 4, 4, 4, 4}
var span30Id = [8]byte{4, 5, 4, 4, 4, 4, 4, 4}

var trace0Id = [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
var trace1Id = [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
var trace2Id = [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
var trace3Id = [16]byte{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}

func GenerateTracesTreesAndOrphans() ptrace.Traces {
	td := ptrace.NewTraces()
	rs0 := td.ResourceSpans().AppendEmpty()
	rs0ils0 := rs0.ScopeSpans().AppendEmpty()
	span001 := rs0ils0.Spans().AppendEmpty()
	span000 := rs0ils0.Spans().AppendEmpty()
	span30 := rs0ils0.Spans().AppendEmpty()
	rs1 := td.ResourceSpans().AppendEmpty()
	rs1ils0 := rs1.ScopeSpans().AppendEmpty()
	span22 := rs1ils0.Spans().AppendEmpty()
	span21 := rs1ils0.Spans().AppendEmpty()
	span10 := rs1ils0.Spans().AppendEmpty()
	span01 := rs1ils0.Spans().AppendEmpty()
	span00 := rs1ils0.Spans().AppendEmpty()
	rs2 := td.ResourceSpans().AppendEmpty()
	rs2ils0 := rs2.ScopeSpans().AppendEmpty()
	span1 := rs2ils0.Spans().AppendEmpty()
	span0 := rs2ils0.Spans().AppendEmpty()
	span002 := rs2ils0.Spans().AppendEmpty()
	span3 := rs2ils0.Spans().AppendEmpty()

	// set error statuses
	status21 := span21.Status()
	status21.SetCode(ptrace.StatusCodeError)

	status000 := span000.Status()
	status000.SetCode(ptrace.StatusCodeError)

	status001 := span001.Status()
	status001.SetCode(ptrace.StatusCodeError)

	// set traces
	trace0 := pcommon.TraceID(trace0Id)
	trace1 := pcommon.TraceID(trace1Id)
	trace2 := pcommon.TraceID(trace2Id)
	trace3 := pcommon.TraceID(trace3Id)

	span0.SetTraceID(trace0)
	span00.SetTraceID(trace0)
	span01.SetTraceID(trace0)
	span000.SetTraceID(trace0)
	span001.SetTraceID(trace0)
	span002.SetTraceID(trace0)

	span1.SetTraceID(trace1)
	span10.SetTraceID(trace1)

	span21.SetTraceID(trace2)
	span22.SetTraceID(trace2)

	span3.SetTraceID(trace3)
	span30.SetTraceID(trace3)

	// set span ids for trace 0 - it's tree

	span0.SetSpanID(span0Id)
	span0.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0)))
	span0.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 10000)))

	span00.SetSpanID(span00Id)
	span00.SetParentSpanID(span0.SpanID())
	span00.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 100)))
	span00.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 4900)))

	span01.SetSpanID(span01Id)
	span01.SetParentSpanID(span0.SpanID())
	span01.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 5100)))
	span01.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 9900)))

	span000.SetSpanID(span000Id)
	span000.SetParentSpanID(span00.SpanID())
	span000.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 200)))
	span000.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 2000)))

	span001.SetSpanID(span001Id)
	span001.SetParentSpanID(span00.SpanID())
	span001.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 2100)))
	span001.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 3800)))

	span002.SetSpanID(span002Id)
	span002.SetParentSpanID(span00.SpanID())
	span002.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 4000)))
	span002.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 4800)))

	// set span ids for trace 1
	span1.SetSpanID(span1Id)
	span1.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 10000)))
	span1.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 20000)))
	span10.SetSpanID(span10Id)
	span10.SetParentSpanID(span1.SpanID())
	span10.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 10100)))
	span10.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 19900)))

	// set span ids for trace 2 - there is no parent
	span21.SetSpanID(span21Id)
	span21.SetParentSpanID(span21PId)
	span21.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 21000)))
	span21.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 22000)))
	span22.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 23000)))
	span22.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 24000)))

	span22.SetSpanID(span22Id)
	span22.SetParentSpanID(span22PId)

	// set spans for trace 3 - parent starts later and starts sooner than child
	// set span ids for trace 1
	span3.SetSpanID(span3Id)
	span3.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 40100)))
	span3.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 49900)))
	span30.SetSpanID(span30Id)
	span30.SetParentSpanID(span3.SpanID())
	span30.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 40000)))
	span30.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 50000)))

	// set resources
	res0 := rs0.Resource()
	res0.Attributes().PutStr("service.name", "sAAA")
	res0.Attributes().PutStr("service.namespace", "snAAA")
	res1 := rs1.Resource()
	res1.Attributes().PutStr("service.name", "sBBB")
	res1.Attributes().PutStr("service.namespace", "snBBB")
	res2 := rs2.Resource()
	res2.Attributes().PutStr("service.name", "sCCC")
	res2.Attributes().PutStr("service.namespace", "snCCC")

	return td
}

func generateSimpleEvent(
	traceID string,
	spanID string,
	parentID string,
	status ptrace.Status,
	start int64,
	end int64,
	serviceName string,
) *add_events.Event {
	attrs := map[string]interface{}{
		"sca:schemVer": 1,
		"sca:schema":   "tracing",
		"sca:type":     "span",

		"name": "",
		"kind": "unspecified",

		"start_time_unix_nano": fmt.Sprintf("%d", start),
		"end_time_unix_nano":   fmt.Sprintf("%d", end),
		"duration_nano":        fmt.Sprintf("%d", end-start),

		"span_id":        spanID,
		"trace_id":       traceID,
		"status_code":    strings.ToLower(status.Code().String()),
		"status_message": status.Message(),

		"resource_name": serviceName,
		"resource_type": "service",
	}
	if parentID != "" {
		attrs["parent_span_id"] = parentID
	}

	return &add_events.Event{
		Thread: "TT",
		Log:    "LT",
		Sev:    9,
		Ts:     fmt.Sprintf("%d", start),
		Attrs:  attrs,
	}
}

func TestBuildEventsFromTracesTrees(t *testing.T) {
	traces := GenerateTracesTreesAndOrphans()
	was := buildEventsFromTraces(traces)

	statusUnset := ptrace.NewStatus()
	statusError := ptrace.NewStatus()
	statusError.SetCode(ptrace.StatusCodeError)

	expected := []*add_events.EventBundle{
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0102030101010101", "0102010101010101", statusError, 2100, 3800, "sAAA"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0102020101010101", "0102010101010101", statusError, 200, 2000, "sAAA"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("04040404040404040404040404040404", "0405040404040404", "0404040404040404", statusUnset, 40000, 50000, "sAAA"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("03030303030303030303030303030303", "0304030303030303", "0306030303030303", statusUnset, 23000, 24000, "sBBB"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("03030303030303030303030303030303", "0303030303030303", "0305030303030303", statusError, 21000, 22000, "sBBB"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("02020202020202020202020202020202", "0203020202020202", "0202020202020202", statusUnset, 10100, 19900, "sBBB"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0103010101010101", "0101010101010101", statusUnset, 5100, 9900, "sBBB"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0102010101010101", "0101010101010101", statusUnset, 100, 4900, "sBBB"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("02020202020202020202020202020202", "0202020202020202", "", statusUnset, 10000, 20000, "sCCC"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0101010101010101", "", statusUnset, 0, 10000, "sCCC"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("01010101010101010101010101010101", "0102040101010101", "0102010101010101", statusUnset, 4000, 4800, "sCCC"),
			Thread: testTThread,
			Log:    testTLog,
		},
		{
			Event:  generateSimpleEvent("04040404040404040404040404040404", "0404040404040404", "", statusUnset, 40100, 49900, "sCCC"),
			Thread: testTThread,
			Log:    testTLog,
		},
	}

	assert.Equal(t, expected, was)
}

func TestUpdateResource(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]any
		expected map[string]interface{}
	}{
		{
			name:     "with_service.name",
			resource: map[string]any{"service.name": "foo"},
			expected: map[string]interface{}{resourceName: "foo", resourceType: string(Service)},
		},
		{
			name:     "without_service.name",
			resource: map[string]any{"service.bar": "foo"},
			expected: map[string]interface{}{resourceName: "", resourceType: string(Service)},
		},
		{
			name:     "with_process.pid",
			resource: map[string]any{"process.pid": "bar"},
			expected: map[string]interface{}{resourceName: "bar", resourceType: string(Process)},
		},
		{
			name:     "prefer_service",
			resource: map[string]any{"service.bar": "foo", "process.pid": "bar"},
			expected: map[string]interface{}{resourceName: "", resourceType: string(Service)},
		},
		{
			name:     "empty",
			resource: map[string]any{},
			expected: map[string]interface{}{resourceName: "", resourceType: string(Process)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			attrs := make(map[string]interface{})
			updateResource(attrs, tt.resource)

			assert.Equal(t, tt.expected, attrs, tt.name)
		})
	}
}
