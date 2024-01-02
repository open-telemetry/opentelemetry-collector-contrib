// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const timeFormatString = "2006-01-02T15:04:05.000Z07:00"

func TestLogsExporter(t *testing.T) {
	lr := testdata.GenerateLogsOneLogRecord()
	ld := lr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	type args struct {
		ld plog.Logs
	}
	tests := []struct {
		name string
		args args
		want testutil.JSONLogs
	}{
		{
			name: "message",
			args: args{
				ld: lr,
			},

			want: testutil.JSONLogs{
				{
					"message":              ld.Body().AsString(),
					"app":                  "server",
					"instance_num":         "1",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
					"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
					"ddtags":               "otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
					"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
			},
		},
		{
			name: "message-attribute",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsOneLogRecord()
					ldd := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					ldd.Attributes().PutStr("message", "hello")
					return lrr
				}(),
			},

			want: testutil.JSONLogs{
				{
					"message":              "hello",
					"app":                  "server",
					"instance_num":         "1",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
					"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
					"ddtags":               "otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
					"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
			},
		},
		{
			name: "ddtags",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsOneLogRecord()
					ldd := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					ldd.Attributes().PutStr("ddtags", "tag1:true")
					return lrr
				}(),
			},

			want: testutil.JSONLogs{
				{
					"message":              ld.Body().AsString(),
					"app":                  "server",
					"instance_num":         "1",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
					"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
					"ddtags":               "tag1:true,otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
					"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
			},
		},
		{
			name: "ddtags submits same tags",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsTwoLogRecordsSameResource()
					ldd := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					ldd.Attributes().PutStr("ddtags", "tag1:true")
					ldd2 := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1)
					ldd2.Attributes().PutStr("ddtags", "tag1:true")
					return lrr
				}(),
			},

			want: testutil.JSONLogs{
				{
					"message":              ld.Body().AsString(),
					"app":                  "server",
					"instance_num":         "1",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
					"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
					"ddtags":               "tag1:true,otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
					"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
				{
					"message":              "something happened",
					"env":                  "dev",
					"customer":             "acme",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"ddtags":               "tag1:true,otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
			},
		},
		{
			name: "ddtags submits different tags",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsTwoLogRecordsSameResource()
					ldd := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					ldd.Attributes().PutStr("ddtags", "tag1:true")
					ldd2 := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1)
					ldd2.Attributes().PutStr("ddtags", "tag2:true")
					return lrr
				}(),
			},

			want: testutil.JSONLogs{
				{
					"message":              ld.Body().AsString(),
					"app":                  "server",
					"instance_num":         "1",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
					"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
					"ddtags":               "tag1:true,otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
					"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
				{
					"message":              "something happened",
					"env":                  "dev",
					"customer":             "acme",
					"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
					"status":               "Info",
					"ddtags":               "tag2:true,otel_source:datadog_exporter",
					"otel.severity_text":   "Info",
					"otel.severity_number": "9",
					"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
					"resource-attr":        "resource-attr-val-1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.DatadogLogServerMock()
			defer server.Close()
			cfg := &Config{
				Metrics: MetricsConfig{
					TCPAddr: confignet.TCPAddr{
						Endpoint: server.URL,
					},
				},
				Logs: LogsConfig{
					TCPAddr: confignet.TCPAddr{
						Endpoint: server.URL,
					},
				},
			}

			params := exportertest.NewNopCreateSettings()
			f := NewFactory()
			ctx := context.Background()
			exp, err := f.CreateLogsExporter(ctx, params, cfg)
			require.NoError(t, err)
			require.NoError(t, exp.ConsumeLogs(ctx, tt.args.ld))
			assert.Equal(t, tt.want, server.LogsData)
		})
	}

}

// traceIDToUint64 converts 128bit traceId to 64 bit uint64
func traceIDToUint64(b [16]byte) uint64 {
	return binary.BigEndian.Uint64(b[len(b)-8:])
}

// spanIDToUint64 converts byte array to uint64
func spanIDToUint64(b [8]byte) uint64 {
	return binary.BigEndian.Uint64(b[:])
}
