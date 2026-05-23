// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type traceEncoderMock struct {
	traces []traceSignal
}

func (e *traceEncoderMock) Encode(v any) error {
	e.traces = append(e.traces, v.(traceSignal))
	return nil
}

func TestConvertTraces(t *testing.T) {
	type args struct {
		td      ptrace.Traces
		encoder Encoder
	}
	tests := []struct {
		name    string
		args    args
		want    []traceSignal
		wantErr bool
	}{
		{
			name: "empty traces",
			args: args{
				td:      ptrace.NewTraces(),
				encoder: &traceEncoderMock{traces: []traceSignal{}},
			},
			want: []traceSignal{},
		},
		{
			name: "basic trace",
			args: args{
				td: func() ptrace.Traces {
					td := ptrace.NewTraces()
					resourceSpans := td.ResourceSpans().AppendEmpty()
					resourceSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
					scopeSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeSpans.Scope().SetName("test-scope")
					scopeSpans.Scope().SetVersion("1.0.0")
					span := scopeSpans.Spans().AppendEmpty()
					span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					span.SetParentSpanID(pcommon.SpanID([8]byte{9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetName("test-span")
					span.SetKind(ptrace.SpanKindServer)
					span.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					span.SetEndTimestamp(pcommon.Timestamp(1719158401000000000))
					span.Status().SetCode(ptrace.StatusCodeOk)
					span.Status().SetMessage("success")
					return td
				}(),
				encoder: &traceEncoderMock{traces: []traceSignal{}},
			},
			want: []traceSignal{
				{
					ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
					ResourceAttributes: map[string]string{},
					ServiceName:        "",
					ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.20.0",
					ScopeName:          "test-scope",
					ScopeVersion:       "1.0.0",
					ScopeAttributes:    map[string]string{},
					TraceID:            "0102030405060708090a0b0c0d0e0f10",
					SpanID:             "0102030405060708",
					ParentSpanID:       "090a0b0c0d0e0f10",
					TraceState:         "",
					TraceFlags:         0,
					SpanName:           "test-span",
					SpanKind:           "Server",
					SpanAttributes:     map[string]string{},
					StartTime:          "2024-06-23T16:00:00Z",
					EndTime:            "2024-06-23T16:00:01Z",
					Duration:           1000000000,
					StatusCode:         "Ok",
					StatusMessage:      "success",
					EventsTimestamp:    []string{},
					EventsName:         []string{},
					EventsAttributes:   []map[string]string{},
					LinksTraceID:       []string{},
					LinksSpanID:        []string{},
					LinksTraceState:    []string{},
					LinksAttributes:    []map[string]string{},
				},
			},
		},
		{
			name: "trace with attributes",
			args: args{
				td: func() ptrace.Traces {
					td := ptrace.NewTraces()
					resourceSpans := td.ResourceSpans().AppendEmpty()
					resourceSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceSpans.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")
					scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
					scopeSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeSpans.Scope().SetName("test-scope")
					scopeSpans.Scope().SetVersion("1.0.0")
					scopeSpans.Scope().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
					span := scopeSpans.Spans().AppendEmpty()
					span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					span.SetParentSpanID(pcommon.SpanID([8]byte{9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetName("test-span")
					span.SetKind(ptrace.SpanKindClient)
					span.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					span.SetEndTimestamp(pcommon.Timestamp(1719158401000000000))
					span.Status().SetCode(ptrace.StatusCodeError)
					span.Status().SetMessage("connection failed")
					span.Attributes().PutStr("http.method", "GET")
					span.Attributes().PutStr("http.url", "http://example.com")
					return td
				}(),
				encoder: &traceEncoderMock{traces: []traceSignal{}},
			},
			want: []traceSignal{
				{
					ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
					ResourceAttributes: map[string]string{
						"service.name": "test-service",
						"environment":  "production",
					},
					ServiceName:    "test-service",
					ScopeSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
					ScopeName:      "test-scope",
					ScopeVersion:   "1.0.0",
					ScopeAttributes: map[string]string{
						"telemetry.sdk.name": "opentelemetry",
					},
					TraceID:      "0102030405060708090a0b0c0d0e0f10",
					SpanID:       "0102030405060708",
					ParentSpanID: "090a0b0c0d0e0f10",
					TraceState:   "",
					TraceFlags:   0,
					SpanName:     "test-span",
					SpanKind:     "Client",
					SpanAttributes: map[string]string{
						"http.method": "GET",
						"http.url":    "http://example.com",
					},
					StartTime:        "2024-06-23T16:00:00Z",
					EndTime:          "2024-06-23T16:00:01Z",
					Duration:         1000000000,
					StatusCode:       "Error",
					StatusMessage:    "connection failed",
					EventsTimestamp:  []string{},
					EventsName:       []string{},
					EventsAttributes: []map[string]string{},
					LinksTraceID:     []string{},
					LinksSpanID:      []string{},
					LinksTraceState:  []string{},
					LinksAttributes:  []map[string]string{},
				},
			},
		},
		{
			name: "trace with events and links",
			args: args{
				td: func() ptrace.Traces {
					td := ptrace.NewTraces()
					resourceSpans := td.ResourceSpans().AppendEmpty()
					resourceSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceSpans.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
					scopeSpans.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeSpans.Scope().SetName("test-scope")
					scopeSpans.Scope().SetVersion("1.0.0")
					span := scopeSpans.Spans().AppendEmpty()
					span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					span.SetName("test-span")
					span.SetKind(ptrace.SpanKindInternal)
					span.SetStartTimestamp(pcommon.Timestamp(1719158400000000000))
					span.SetEndTimestamp(pcommon.Timestamp(1719158401000000000))
					span.Status().SetCode(ptrace.StatusCodeOk)

					// Add span event
					event := span.Events().AppendEmpty()
					event.SetName("exception")
					event.SetTimestamp(pcommon.Timestamp(1719158400500000000))
					event.Attributes().PutStr("exception.type", "RuntimeException")
					event.Attributes().PutStr("exception.message", "Something went wrong")

					// Add span link
					link := span.Links().AppendEmpty()
					link.SetTraceID(pcommon.TraceID([16]byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}))
					link.SetSpanID(pcommon.SpanID([8]byte{17, 18, 19, 20, 21, 22, 23, 24}))
					link.TraceState().FromRaw("sampled=true")
					link.Attributes().PutStr("link.type", "child")

					return td
				}(),
				encoder: &traceEncoderMock{traces: []traceSignal{}},
			},
			want: []traceSignal{
				{
					ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
					ResourceAttributes: map[string]string{
						"service.name": "test-service",
					},
					ServiceName:     "test-service",
					ScopeSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
					ScopeName:       "test-scope",
					ScopeVersion:    "1.0.0",
					ScopeAttributes: map[string]string{},
					TraceID:         "0102030405060708090a0b0c0d0e0f10",
					SpanID:          "0102030405060708",
					ParentSpanID:    "",
					TraceState:      "",
					TraceFlags:      0,
					SpanName:        "test-span",
					SpanKind:        "Internal",
					SpanAttributes:  map[string]string{},
					StartTime:       "2024-06-23T16:00:00Z",
					EndTime:         "2024-06-23T16:00:01Z",
					Duration:        1000000000,
					StatusCode:      "Ok",
					StatusMessage:   "",
					EventsTimestamp: []string{"2024-06-23T16:00:00.5Z"},
					EventsName:      []string{"exception"},
					EventsAttributes: []map[string]string{
						{
							"exception.type":    "RuntimeException",
							"exception.message": "Something went wrong",
						},
					},
					LinksTraceID:    []string{"1112131415161718191a1b1c1d1e1f20"},
					LinksSpanID:     []string{"1112131415161718"},
					LinksTraceState: []string{"sampled=true"},
					LinksAttributes: []map[string]string{
						{
							"link.type": "child",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertTraces(tt.args.td, tt.args.encoder); (err != nil) != tt.wantErr {
				t.Errorf("ConvertTraces() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.encoder.(*traceEncoderMock).traces, tt.want) {
				t.Errorf("ConvertTraces() traces = %v, want %v", tt.args.encoder.(*traceEncoderMock).traces, tt.want)
			}
		})
	}
}
