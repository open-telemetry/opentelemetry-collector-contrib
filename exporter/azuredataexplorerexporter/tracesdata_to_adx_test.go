// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_mapToAdxTrace(t *testing.T) {
	epoch, _ := time.Parse("2006-01-02T15:04:05.999999999Z07:00", "1970-01-01T00:00:00.000000000Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339Nano)
	tmap := make(map[string]any)
	tmap["key"] = "value"
	tmap[hostkey] = testhost

	spanID := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	tests := []struct {
		name             string             // name of the test
		spanDatafn       func() ptrace.Span // function that generates the required spans for the test
		resourceFn       func() pcommon.Resource
		insScopeFn       func() pcommon.InstrumentationScope
		expectedAdxTrace *adxTrace
	}{
		{
			name: "valid",
			spanDatafn: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().PutStr("traceAttribKey", "traceAttribVal")

				return span
			},
			resourceFn: newDummyResource,
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &adxTrace{
				TraceID:            "00000000000000000000000000000064",
				SpanID:             "0000000000000032",
				ParentID:           "",
				SpanName:           "spanname",
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_SERVER",
				StartTime:          tstr,
				EndTime:            tstr,
				ResourceAttributes: tmap,
				TraceAttributes:    newMapFromAttr(`{"traceAttribKey":"traceAttribVal", "scope.name":"testscope", "scope.version":"1.0"}`),
				Events:             getEmptyEvents(),
				Links:              getEmptyLinks(),
			},
		},
		{
			name: "No data",
			spanDatafn: func() ptrace.Span {
				span := ptrace.NewSpan()
				return span
			},
			resourceFn: pcommon.NewResource,
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &adxTrace{
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_UNSPECIFIED",
				StartTime:          defaultTime,
				EndTime:            defaultTime,
				ResourceAttributes: newMapFromAttr(`{}`),
				TraceAttributes:    newMapFromAttr(`{"scope.name":"testscope", "scope.version":"1.0"}`),
				Events:             getEmptyEvents(),
				Links:              getEmptyLinks(),
			},
		},
		{
			name: "with_events_links",
			spanDatafn: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().PutStr("traceAttribKey", "traceAttribVal")
				event := span.Events().AppendEmpty()
				event.SetName("eventName")
				event.SetTimestamp(ts)
				event.Attributes().PutStr("eventkey", "eventvalue")

				link := span.Links().AppendEmpty()
				link.SetSpanID(pcommon.SpanID(spanID))
				link.SetTraceID(pcommon.TraceID(traceID))
				link.TraceState().FromRaw("")

				return span
			},
			resourceFn: newDummyResource,
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &adxTrace{
				TraceID:            "00000000000000000000000000000064",
				SpanID:             "0000000000000032",
				ParentID:           "",
				SpanName:           "spanname",
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_SERVER",
				StartTime:          tstr,
				EndTime:            tstr,
				ResourceAttributes: tmap,
				TraceAttributes:    newMapFromAttr(`{"traceAttribKey":"traceAttribVal", "scope.name":"testscope", "scope.version":"1.0"}`),
				Events: []*event{
					{
						EventName:       "eventName",
						EventAttributes: newMapFromAttr(`{"eventkey": "eventvalue"}`),
						Timestamp:       tstr,
					},
				},
				Links: []*link{{
					TraceID:            "00000000000000000000000000000064",
					SpanID:             "0000000000000032",
					TraceState:         "",
					SpanLinkAttributes: newMapFromAttr(`{}`),
				}},
			},
		},
		{
			name: "with_status_message_for_error",
			spanDatafn: func() ptrace.Span {
				span := ptrace.NewSpan()
				span.SetName("spanname-status-message")
				span.Status().SetCode(ptrace.StatusCodeError)
				span.Status().SetMessage("An error occurred")
				span.SetTraceID(pcommon.TraceID(traceID))
				span.SetSpanID(pcommon.SpanID(spanID))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().PutStr("traceAttribKey", "traceAttribVal")
				event := span.Events().AppendEmpty()
				event.SetName("eventName")
				event.SetTimestamp(ts)
				event.Attributes().PutStr("eventkey", "eventvalue")

				link := span.Links().AppendEmpty()
				link.SetSpanID(pcommon.SpanID(spanID))
				link.SetTraceID(pcommon.TraceID(traceID))
				link.TraceState().FromRaw("")
				return span
			},
			resourceFn: newDummyResource,
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &adxTrace{
				TraceID:            "00000000000000000000000000000064",
				SpanID:             "0000000000000032",
				ParentID:           "",
				SpanName:           "spanname-status-message",
				SpanStatus:         "STATUS_CODE_ERROR",
				SpanStatusMessage:  "An error occurred",
				SpanKind:           "SPAN_KIND_SERVER",
				StartTime:          tstr,
				EndTime:            tstr,
				ResourceAttributes: tmap,
				TraceAttributes:    newMapFromAttr(`{"traceAttribKey":"traceAttribVal", "scope.name":"testscope", "scope.version":"1.0"}`),
				Events: []*event{
					{
						EventName:       "eventName",
						EventAttributes: newMapFromAttr(`{"eventkey": "eventvalue"}`),
						Timestamp:       tstr,
					},
				},
				Links: []*link{{
					TraceID:            "00000000000000000000000000000064",
					SpanID:             "0000000000000032",
					TraceState:         "",
					SpanLinkAttributes: newMapFromAttr(`{}`),
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.expectedAdxTrace
			got := mapToAdxTrace(tt.resourceFn(), tt.insScopeFn(), tt.spanDatafn())
			require.NotNil(t, got)
			assert.Equal(t, want, got)
		})
	}
}

func getEmptyEvents() []*event {
	return []*event{}
}

func getEmptyLinks() []*link {
	return []*link{}
}
