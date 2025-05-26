// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalking

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func TestSetInternalSpanStatus(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agent.SpanObject
		dest   ptrace.Status
		code   ptrace.StatusCode
	}{
		{
			name: "StatusCodeError",
			swSpan: &agent.SpanObject{
				IsError: true,
			},
			dest: generateTracesOneEmptyResourceSpans().Status(),
			code: ptrace.StatusCodeError,
		},
		{
			name: "StatusCodeOk",
			swSpan: &agent.SpanObject{
				IsError: false,
			},
			dest: generateTracesOneEmptyResourceSpans().Status(),
			code: ptrace.StatusCodeOk,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setInternalSpanStatus(test.swSpan, test.dest)
			assert.Equal(t, test.code, test.dest.Code())
		})
	}
}

func TestSwKvPairsToInternalAttributes(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agent.SegmentObject
		dest   ptrace.Span
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-2",
			swSpan: mockGrpcTraceSegment(2),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			swKvPairsToInternalAttributes(test.swSpan.GetSpans()[0].Tags, test.dest.Attributes())
			assert.Len(t, test.swSpan.GetSpans()[0].Tags, test.dest.Attributes().Len())
			for _, tag := range test.swSpan.GetSpans()[0].Tags {
				value, _ := test.dest.Attributes().Get(tag.Key)
				assert.Equal(t, tag.Value, value.AsString())
			}
		})
	}
}

func TestSwProtoToTraces(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agent.SegmentObject
		dest   ptrace.Traces
		code   ptrace.StatusCode
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			td := ProtoToTraces(test.swSpan)
			assert.Equal(t, 1, td.ResourceSpans().Len())
			assert.Equal(t, 2, td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
		})
	}
}

func TestSwReferencesToSpanLinks(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agent.SegmentObject
		dest   ptrace.Span
	}{
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-2",
			swSpan: mockGrpcTraceSegment(2),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			swReferencesToSpanLinks(test.swSpan.GetSpans()[0].Refs, test.dest.Links())
			assert.Equal(t, 1, test.dest.Links().Len())
		})
	}
}

func TestSwLogsToSpanEvents(t *testing.T) {
	tests := []struct {
		name   string
		swSpan *agent.SegmentObject
		dest   ptrace.Span
	}{
		{
			name:   "mock-sw-swgment-0",
			swSpan: mockGrpcTraceSegment(0),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
		{
			name:   "mock-sw-swgment-1",
			swSpan: mockGrpcTraceSegment(1),
			dest:   generateTracesOneEmptyResourceSpans(),
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq := strconv.Itoa(index)
			swLogsToSpanEvents(test.swSpan.GetSpans()[0].Logs, test.dest.Events())
			assert.Equal(t, 1, test.dest.Events().Len())
			assert.Equal(t, "logs", test.dest.Events().At(0).Name())
			logValue, _ := test.dest.Events().At(0).Attributes().Get("log-key" + seq)
			assert.Equal(t, "log-value"+seq, logValue.AsString())
		})
	}
}

func Test_stringToTraceID(t *testing.T) {
	type args struct {
		traceID string
	}
	tests := []struct {
		name          string
		segmentObject args
		want          pcommon.TraceID
	}{
		{
			name:          "mock-sw-normal-trace-id-rfc4122v4",
			segmentObject: args{traceID: "de5980b8-fce3-4a37-aab9-b4ac3af7eedd"},
			want:          [16]byte{222, 89, 128, 184, 252, 227, 74, 55, 170, 185, 180, 172, 58, 247, 238, 221},
		},
		{
			name:          "mock-sw-normal-trace-id-rfc4122",
			segmentObject: args{traceID: "de5980b8fce34a37aab9b4ac3af7eedd"},
			want:          [16]byte{222, 89, 128, 184, 252, 227, 74, 55, 170, 185, 180, 172, 58, 247, 238, 221},
		},
		{
			name:          "mock-sw-trace-id-length-shorter",
			segmentObject: args{traceID: "de59"},
			want:          [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:          "mock-sw-trace-id-length-java-agent",
			segmentObject: args{traceID: "de5980b8fce34a37aab9b4ac3af7eedd.1.16563474296430001"},
			want:          [16]byte{222, 89, 128, 184, 253, 227, 74, 55, 27, 228, 27, 205, 94, 47, 212, 221},
		},
		{
			name:          "mock-sw-trace-id-illegal",
			segmentObject: args{traceID: ".,<>?/-=+MNop"},
			want:          [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := swTraceIDToTraceID(tt.segmentObject.traceID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_stringToTraceID_Unique(t *testing.T) {
	type args struct {
		traceID string
	}
	tests := []struct {
		name          string
		segmentObject args
	}{
		{
			name:          "mock-sw-trace-id-unique-1",
			segmentObject: args{traceID: "de5980b8fce34a37aab9b4ac3af7eedd.133.16563474296430001"},
		},
		{
			name:          "mock-sw-trace-id-unique-2",
			segmentObject: args{traceID: "de5980b8fce34a37aab9b4ac3af7eedd.133.16534574123430001"},
		},
	}

	var results [2][16]byte
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(_ *testing.T) {
			got := swTraceIDToTraceID(tt.segmentObject.traceID)
			results[i] = got
		})
	}
	assert.NotEqual(t, tests[0].segmentObject.traceID, t, tests[1].segmentObject.traceID)
	assert.NotEqual(t, results[0], results[1])
}

func Test_segmentIdToSpanId(t *testing.T) {
	type args struct {
		segmentID string
		spanID    uint32
	}
	tests := []struct {
		name string
		args args
		want pcommon.SpanID
	}{
		{
			name: "mock-sw-span-id-normal",
			args: args{segmentID: "4f2f27748b8e44ecaf18fe0347194e86.33.16560607369950066", spanID: 123},
			want: [8]byte{233, 196, 85, 168, 37, 66, 48, 106},
		},
		{
			name: "mock-sw-span-id-python-agent",
			args: args{segmentID: "4f2f27748b8e44ecaf18fe0347194e86", spanID: 123},
			want: [8]byte{155, 55, 217, 119, 204, 151, 10, 106},
		},
		{
			name: "mock-sw-span-id-short",
			args: args{segmentID: "16560607369950066", spanID: 12},
			want: [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "mock-sw-span-id-illegal-1",
			args: args{segmentID: "1", spanID: 2},
			want: [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "mock-sw-span-id-illegal-char",
			args: args{segmentID: ".,<>?/-=+MNop", spanID: 2},
			want: [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segmentIDToSpanID(tt.args.segmentID, tt.args.spanID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_segmentIdToSpanId_Unique(t *testing.T) {
	type args struct {
		segmentID string
		spanID    uint32
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "mock-sw-span-id-unique-1",
			args: args{segmentID: "4f2f27748b8e44ecaf18fe0347194e86.33.16560607369950066", spanID: 123},
		},
		{
			name: "mock-sw-span-id-unique-2",
			args: args{segmentID: "4f2f27748b8e44ecaf18fe0347194e86.33.16560607369950066", spanID: 1},
		},
	}
	var results [2][8]byte
	for i := 0; i < 2; i++ {
		tt := tests[i]
		t.Run(tt.name, func(_ *testing.T) {
			got := segmentIDToSpanID(tt.args.segmentID, tt.args.spanID)
			results[i] = got
		})
	}

	assert.NotEqual(t, results[0], results[1])
}

func Test_swSpanToSpan_ParentSpanId(t *testing.T) {
	type args struct {
		span *agent.SpanObject
	}
	tests := []struct {
		name string
		args args
		want pcommon.SpanID
	}{
		{
			name: "mock-sw-span-with-parent-segment",
			args: args{span: &agent.SpanObject{
				ParentSpanId: -1,
				Refs: []*agent.SegmentReference{{
					ParentTraceSegmentId: "4f2f27748b8e44ecaf18fe0347194e86.33.16560607369950066",
					ParentSpanId:         123,
				}},
			}},
			want: [8]byte{233, 196, 85, 168, 37, 66, 48, 106},
		},
		{
			name: "mock-sw-span-without-parent-segment",
			args: args{span: &agent.SpanObject{Refs: []*agent.SegmentReference{{
				ParentSpanId: -1,
			}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := ptrace.NewSpan()
			swSpanToSpan("de5980b8-fce3-4a37-aab9-b4ac3af7eedd", "", tt.args.span, dest)
			assert.Equal(t, tt.want, dest.ParentSpanID())
		})
	}
}

func generateTracesOneEmptyResourceSpans() ptrace.Span {
	td := ptrace.NewTraces()
	resourceSpan := td.ResourceSpans().AppendEmpty()
	il := resourceSpan.ScopeSpans().AppendEmpty()
	il.Spans().AppendEmpty()
	return il.Spans().At(0)
}

func mockGrpcTraceSegment(sequence int) *agent.SegmentObject {
	seq := strconv.Itoa(sequence)
	return &agent.SegmentObject{
		TraceId:         "trace" + seq,
		TraceSegmentId:  "trace-segment" + seq,
		Service:         "demo-segmentReportService" + seq,
		ServiceInstance: "demo-instance" + seq,
		IsSizeLimited:   false,
		Spans: []*agent.SpanObject{
			{
				SpanId:        1,
				ParentSpanId:  0,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 10,
				OperationName: "operation" + seq,
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Entry,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   1,
				IsError:       false,
				SkipAnalysis:  false,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "mock-key" + seq,
						Value: "mock-value" + seq,
					},
				},
				Logs: []*agent.Log{
					{
						Time: time.Now().Unix(),
						Data: []*common.KeyStringValuePair{
							{
								Key:   "log-key" + seq,
								Value: "log-value" + seq,
							},
						},
					},
				},
				Refs: []*agent.SegmentReference{
					{
						RefType:                  agent.RefType_CrossThread,
						TraceId:                  "trace" + seq,
						ParentTraceSegmentId:     "parent-trace-segment" + seq,
						ParentSpanId:             0,
						ParentService:            "parent" + seq,
						ParentServiceInstance:    "parent" + seq,
						ParentEndpoint:           "parent" + seq,
						NetworkAddressUsedAtPeer: "127.0.0.1:6666",
					},
				},
			},
			{
				SpanId:        2,
				ParentSpanId:  1,
				StartTime:     time.Now().Unix(),
				EndTime:       time.Now().Unix() + 20,
				OperationName: "operation" + seq,
				Peer:          "127.0.0.1:6666",
				SpanType:      agent.SpanType_Local,
				SpanLayer:     agent.SpanLayer_Http,
				ComponentId:   2,
				IsError:       false,
				SkipAnalysis:  false,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "mock-key" + seq,
						Value: "mock-value" + seq,
					},
				},
				Logs: []*agent.Log{
					{
						Time: time.Now().Unix(),
						Data: []*common.KeyStringValuePair{
							{
								Key:   "log-key" + seq,
								Value: "log-value" + seq,
							},
						},
					},
				},
			},
		},
	}
}
