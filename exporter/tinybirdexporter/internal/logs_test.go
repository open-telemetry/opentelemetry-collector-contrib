// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logEncoderMock struct {
	logs []logSignal
}

func (e *logEncoderMock) Encode(v any) error {
	e.logs = append(e.logs, v.(logSignal))
	return nil
}

func TestConvertLogs(t *testing.T) {
	type args struct {
		ld      plog.Logs
		encoder Encoder
	}
	tests := []struct {
		name    string
		args    args
		want    []logSignal
		wantErr bool
	}{
		{
			name: "empty logs",
			args: args{
				ld:      plog.NewLogs(),
				encoder: &logEncoderMock{logs: []logSignal{}},
			},
			want: []logSignal{},
		},
		{
			name: "basic log",
			args: args{
				ld: func() plog.Logs {
					ld := plog.NewLogs()
					resourceLogs := ld.ResourceLogs().AppendEmpty()
					resourceLogs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
					scopeLogs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeLogs.Scope().SetName("test-scope")
					scopeLogs.Scope().SetVersion("1.0.0")
					log := scopeLogs.LogRecords().AppendEmpty()
					log.SetTimestamp(pcommon.Timestamp(1719158400000000000))
					log.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					log.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					log.SetFlags(1)
					log.SetSeverityText("INFO")
					log.SetSeverityNumber(plog.SeverityNumberInfo)
					log.Body().SetStr("log without attributes")
					return ld
				}(),
				encoder: &logEncoderMock{logs: []logSignal{}},
			},
			want: []logSignal{
				{
					ResourceSchemaURL:  "https://opentelemetry.io/schemas/1.20.0",
					ResourceAttributes: map[string]string{},
					ServiceName:        "",
					ScopeSchemaURL:     "https://opentelemetry.io/schemas/1.20.0",
					ScopeAttributes:    map[string]string{},
					ScopeName:          "test-scope",
					ScopeVersion:       "1.0.0",
					Timestamp:          "2024-06-23T16:00:00Z",
					TraceID:            "0102030405060708090a0b0c0d0e0f10",
					SpanID:             "0102030405060708",
					Flags:              1,
					SeverityText:       "INFO",
					SeverityNumber:     9,
					LogAttributes:      map[string]string{},
					Body:               "log without attributes",
				},
			},
		},
		{
			name: "log with attributes",
			args: args{
				ld: func() plog.Logs {
					ld := plog.NewLogs()
					resourceLogs := ld.ResourceLogs().AppendEmpty()
					resourceLogs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := resourceLogs.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")
					scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
					scopeLogs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scopeLogs.Scope().SetName("test-scope")
					scopeLogs.Scope().SetVersion("1.0.0")
					scopeLogs.Scope().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
					log := scopeLogs.LogRecords().AppendEmpty()
					log.SetTimestamp(pcommon.Timestamp(1719158400000000000))
					log.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					log.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					log.SetFlags(1)
					log.SetSeverityText("INFO")
					log.SetSeverityNumber(plog.SeverityNumberInfo)
					log.Body().SetStr("log with attributes")
					log.Attributes().PutStr("http.method", "GET")
					log.Attributes().PutStr("http.url", "http://example.com")
					return ld
				}(),
				encoder: &logEncoderMock{logs: []logSignal{}},
			},
			want: []logSignal{
				{
					ResourceSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
					ResourceAttributes: map[string]string{
						"service.name": "test-service",
						"environment":  "production",
					},
					ServiceName:    "test-service",
					ScopeSchemaURL: "https://opentelemetry.io/schemas/1.20.0",
					ScopeAttributes: map[string]string{
						"telemetry.sdk.name": "opentelemetry",
					},
					ScopeName:      "test-scope",
					ScopeVersion:   "1.0.0",
					Timestamp:      "2024-06-23T16:00:00Z",
					TraceID:        "0102030405060708090a0b0c0d0e0f10",
					SpanID:         "0102030405060708",
					Flags:          1,
					SeverityText:   "INFO",
					SeverityNumber: 9,
					LogAttributes: map[string]string{
						"http.method": "GET",
						"http.url":    "http://example.com",
					},
					Body: "log with attributes",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertLogs(tt.args.ld, tt.args.encoder); (err != nil) != tt.wantErr {
				t.Errorf("ConvertLogs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.encoder.(*logEncoderMock).logs, tt.want) {
				t.Errorf("ConvertLogs() logs = %v, want %v", tt.args.encoder.(*logEncoderMock).logs, tt.want)
			}
		})
	}
}
