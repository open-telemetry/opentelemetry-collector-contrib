// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap/zaptest"
)

func TestTransform(t *testing.T) {
	testLogger := zaptest.NewLogger(t)
	traceID := [16]byte{0x08, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x0, 0x0, 0x0, 0x0, 0x0a}
	var spanID [8]byte
	copy(spanID[:], traceID[8:])
	ddTr := traceIDToUint64(traceID)
	ddSp := spanIDToUint64(spanID)

	type args struct {
		lr  plog.LogRecord
		res pcommon.Resource
	}
	tests := []struct {
		name string
		args args
		want datadogV2.HTTPLogItem
	}{
		{
			// log with an attribute
			name: "basic",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: pcommon.NewResource(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
				},
			},
		},
		{
			// log & resource with attribute
			name: "resource",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					r.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("service:otlp_col,otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
				},
			},
		},
		{
			// appends tags in attributes instead of replacing them
			name: "append tags",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.Attributes().PutStr("ddtags", "foo:bar")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					r.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("service:otlp_col,foo:bar,otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
				},
			},
		},
		{
			// service name from log
			name: "service",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
					"service.name":     "otlp_col",
				},
			},
		},
		{
			name: "trace",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSpanID(spanID)
					l.SetTraceID(traceID)
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
					otelSpanID:         fmt.Sprintf("%x", string(spanID[:])),
					otelTraceID:        fmt.Sprintf("%x", string(traceID[:])),
					ddSpanID:           fmt.Sprintf("%d", ddSp),
					ddTraceID:          fmt.Sprintf("%d", ddTr),
					"service.name":     "otlp_col",
				},
			},
		},
		{
			name: "trace from attributes",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.Attributes().PutStr("spanid", "2e26da881214cd7c")
					l.Attributes().PutStr("traceid", "437ab4d83468c540bb0f3398a39faa59")
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
					otelSpanID:         "2e26da881214cd7c",
					otelTraceID:        "437ab4d83468c540bb0f3398a39faa59",
					ddSpanID:           "3325585652813450620",
					ddTraceID:          "13479048940416379481",
					"service.name":     "otlp_col",
				},
			},
		},
		{
			name: "trace from attributes decode error",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.Attributes().PutStr("spanid", "2e26da881214cd7c")
					l.Attributes().PutStr("traceid", "invalidtraceid")
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "debug",
					otelSeverityNumber: "5",
					otelSpanID:         "2e26da881214cd7c",
					ddSpanID:           "3325585652813450620",
					"service.name":     "otlp_col",
				},
			},
		},
		{
			// here SeverityText should take precedence for log status
			name: "SeverityText",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSpanID(spanID)
					l.SetTraceID(traceID)
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityText("alert")
					l.SetSeverityNumber(5)
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"app":              "test",
					"status":           "alert",
					otelSeverityText:   "alert",
					otelSeverityNumber: "5",
					otelSpanID:         fmt.Sprintf("%x", string(spanID[:])),
					otelTraceID:        fmt.Sprintf("%x", string(traceID[:])),
					ddSpanID:           fmt.Sprintf("%d", ddSp),
					ddTraceID:          fmt.Sprintf("%d", ddTr),
					"service.name":     "otlp_col",
				},
			},
		},
		{
			name: "body",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSpanID(spanID)
					l.SetTraceID(traceID)
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.SetSeverityNumber(13)
					l.Body().SetStr("This is log")
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"message":          "This is log",
					"app":              "test",
					"status":           "warn",
					otelSeverityNumber: "13",
					otelSpanID:         fmt.Sprintf("%x", string(spanID[:])),
					otelTraceID:        fmt.Sprintf("%x", string(traceID[:])),
					ddSpanID:           fmt.Sprintf("%d", ddSp),
					ddTraceID:          fmt.Sprintf("%d", ddTr),
					"service.name":     "otlp_col",
				},
			},
		},
		{
			name: "log-level",
			args: args{
				lr: func() plog.LogRecord {
					l := plog.NewLogRecord()
					l.Attributes().PutStr("app", "test")
					l.SetSpanID(spanID)
					l.SetTraceID(traceID)
					l.Attributes().PutStr(conventions.AttributeServiceName, "otlp_col")
					l.Attributes().PutStr("level", "error")
					l.Body().SetStr("This is log")
					return l
				}(),
				res: func() pcommon.Resource {
					r := pcommon.NewResource()
					return r
				}(),
			},
			want: datadogV2.HTTPLogItem{
				Ddtags:  datadog.PtrString("otel_source:datadog_exporter"),
				Message: *datadog.PtrString(""),
				Service: datadog.PtrString("otlp_col"),
				AdditionalProperties: map[string]string{
					"message":      "This is log",
					"app":          "test",
					"status":       "error",
					otelSpanID:     fmt.Sprintf("%x", string(spanID[:])),
					otelTraceID:    fmt.Sprintf("%x", string(traceID[:])),
					ddSpanID:       fmt.Sprintf("%d", ddSp),
					ddTraceID:      fmt.Sprintf("%d", ddTr),
					"service.name": "otlp_col",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Transform(tt.args.lr, tt.args.res, testLogger)

			gs, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
				return
			}
			ws, err := tt.want.MarshalJSON()
			if err != nil {
				t.Fatal(err)
				return
			}
			if !assert.JSONEq(t, string(ws), string(gs)) {
				t.Errorf("Transform() = %v, want %v", string(gs), string(ws))
			}
		})
	}
}

func TestDeriveStatus(t *testing.T) {
	type args struct {
		severity plog.SeverityNumber
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "trace3",
			args: args{
				severity: 3,
			},
			want: logLevelTrace,
		},
		{
			name: "trace4",
			args: args{
				severity: 4,
			},
			want: logLevelTrace,
		},
		{
			name: "debug5",
			args: args{
				severity: 5,
			},
			want: logLevelDebug,
		},
		{
			name: "debug7",
			args: args{
				severity: 7,
			},
			want: logLevelDebug,
		},
		{
			name: "debug8",
			args: args{
				severity: 8,
			},
			want: logLevelDebug,
		},
		{
			name: "info9",
			args: args{
				severity: 9,
			},
			want: logLevelInfo,
		},
		{
			name: "info12",
			args: args{
				severity: 12,
			},
			want: logLevelInfo,
		},
		{
			name: "warn13",
			args: args{
				severity: 13,
			},
			want: logLevelWarn,
		},
		{
			name: "warn16",
			args: args{
				severity: 16,
			},
			want: logLevelWarn,
		},
		{
			name: "error17",
			args: args{
				severity: 17,
			},
			want: logLevelError,
		},
		{
			name: "error20",
			args: args{
				severity: 20,
			},
			want: logLevelError,
		},
		{
			name: "fatal21",
			args: args{
				severity: 21,
			},
			want: logLevelFatal,
		},
		{
			name: "fatal24",
			args: args{
				severity: 24,
			},
			want: logLevelFatal,
		},
		{
			name: "undefined",
			args: args{
				severity: 50,
			},
			want: logLevelError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, statusFromSeverityNumber(tt.args.severity), "derviveDdStatusFromSeverityNumber(%v)", tt.args.severity)
		})
	}
}
