package azuredataexplorerexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func Test_mapToAdxTrace(t *testing.T) {
	logger := zap.NewNop()
	epoch, _ := time.Parse("2006-01-02T15:04:05Z07:00", "1970-01-01T00:00:00Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"
	tmap[hostkey] = testhost

	scpMap := map[string]string{
		"name":    "testscope",
		"version": "1.0",
	}

	spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	tests := []struct {
		name             string             // name of the test
		spanDatafn       func() ptrace.Span // function that generates the
		resourceFn       func() pcommon.Resource
		insScopeFn       func() pcommon.InstrumentationScope
		expectedAdxTrace *AdxTrace
	}{
		{
			name: "valid",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.NewTraceID(traceId))
				span.SetSpanID(pcommon.NewSpanID(spanId))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().InsertString("traceAttribKey", "traceAttribVal")

				return span
			},
			resourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			insScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxTrace: &AdxTrace{
				TraceId:              "00000000000000000000000000000064",
				SpanId:               "0000000000000032",
				ParentId:             "",
				SpanName:             "spanname",
				SpanStatus:           "STATUS_CODE_UNSET",
				SpanKind:             "SPAN_KIND_SERVER",
				StartTime:            tstr,
				EndTime:              tstr,
				ResourceAttributes:   tmap,
				InstrumentationScope: scpMap,
				TraceAttributes:      newMapFromAttr(`{"traceAttribKey":"traceAttribVal"}`),
				Events:               getEmptyEvents(),
				Links:                getEmptyLinks(),
			},
		}, {
			name: "No data",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				return span
			},
			resourceFn: pcommon.NewResource,
			insScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxTrace: &AdxTrace{
				SpanStatus:           "STATUS_CODE_UNSET",
				SpanKind:             "SPAN_KIND_UNSPECIFIED",
				StartTime:            defaultTime,
				EndTime:              defaultTime,
				InstrumentationScope: scpMap,
				ResourceAttributes:   newMapFromAttr(`{}`),
				TraceAttributes:      newMapFromAttr(`{}`),
				Events:               getEmptyEvents(),
				Links:                getEmptyLinks(),
			},
		}, {
			name: "with_events_links",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.NewTraceID(traceId))
				span.SetSpanID(pcommon.NewSpanID(spanId))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().InsertString("traceAttribKey", "traceAttribVal")
				event := span.Events().AppendEmpty()
				event.SetName("eventName")
				event.SetTimestamp(ts)
				event.Attributes().InsertString("eventkey", "eventvalue")

				link := span.Links().AppendEmpty()
				link.SetSpanID(pcommon.NewSpanID(spanId))
				link.SetTraceID(pcommon.NewTraceID(traceId))
				link.SetTraceState(ptrace.TraceStateEmpty)

				return span
			},
			resourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			insScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxTrace: &AdxTrace{
				TraceId:              "00000000000000000000000000000064",
				SpanId:               "0000000000000032",
				ParentId:             "",
				SpanName:             "spanname",
				SpanStatus:           "STATUS_CODE_UNSET",
				SpanKind:             "SPAN_KIND_SERVER",
				StartTime:            tstr,
				EndTime:              tstr,
				ResourceAttributes:   tmap,
				InstrumentationScope: scpMap,
				TraceAttributes:      newMapFromAttr(`{"traceAttribKey":"traceAttribVal"}`),
				Events: []*Event{
					{
						EventName:       "eventName",
						EventAttributes: newMapFromAttr(`{"eventkey": "eventvalue"}`),
						Timestamp:       tstr,
					},
				},
				Links: []*Link{{
					TraceId:            "00000000000000000000000000000064",
					SpanId:             "0000000000000032",
					TraceState:         string(ptrace.TraceStateEmpty),
					SpanLinkAttributes: newMapFromAttr(`{}`),
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.expectedAdxTrace
			got := mapToAdxTrace(tt.resourceFn(), tt.insScopeFn(), tt.spanDatafn(), logger)
			require.NotNil(t, got)
			assert.Equal(t, want, got)

		})
	}

}

func getEmptyEvents() []*Event {
	return []*Event{}

}

func getEmptyLinks() []*Link {
	return []*Link{}
}
