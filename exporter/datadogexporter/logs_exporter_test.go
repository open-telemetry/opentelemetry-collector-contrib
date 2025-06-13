// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions127 "go.opentelemetry.io/collector/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
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
	featuregateErr := featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", false)
	assert.NoError(t, featuregateErr)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.DatadogLogServerMock()
			defer server.Close()
			cfg := &datadogconfig.Config{
				Metrics: datadogconfig.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: server.URL,
					},
				},
				Logs: datadogconfig.LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: server.URL,
					},
				},
			}

			params := exportertest.NewNopSettings(metadata.Type)
			f := NewFactory()
			ctx := context.Background()
			exp, err := f.CreateLogs(ctx, params, cfg)
			require.NoError(t, err)
			require.NoError(t, exp.ConsumeLogs(ctx, tt.args.ld))
			assert.Equal(t, tt.want, server.LogsData)
		})
	}
	featuregateErr = featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", true)
	assert.NoError(t, featuregateErr)
}

func TestLogsAgentExporter(t *testing.T) {
	lr := testdata.GenerateLogsOneLogRecord()
	ld := lr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	type args struct {
		ld    plog.Logs
		retry bool
	}
	tests := []struct {
		name string
		args args
		want testutil.JSONLogs
	}{
		{
			name: "message",
			args: args{
				ld:    lr,
				retry: false,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
			},
		},
		{
			name: "message-attribute",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsOneLogRecord()
					ldd := lrr.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					ldd.Attributes().PutStr("attr", "hello")
					ldd.Attributes().PutStr("service.name", "service")
					ldd.Attributes().PutStr("host.name", "test-host")
					return lrr
				}(),
				retry: false,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
						"attr":                 "hello",
						"service":              "service",
						"service.name":         "service",
						"host.name":            "test-host",
						"hostname":             "test-host",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "otel_source:datadog_exporter",
					"service":  "service",
					"status":   "Info",
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
				retry: false,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "tag1:true,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
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
				retry: false,
			},

			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "tag1:true,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"message":              "something happened",
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
						"env":                  "dev",
						"customer":             "acme",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "tag1:true,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
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
				retry: false,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "tag1:true,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"message":              "something happened",
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
						"env":                  "dev",
						"customer":             "acme",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "tag2:true,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
			},
		},
		{
			name: "message with retry",
			args: args{
				ld:    lr,
				retry: true,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":           testdata.TestLogTime.Format(timeFormatString),
						"app":                  "server",
						"dd.span_id":           fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":          fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"instance_num":         "1",
						"message":              ld.Body().AsString(),
						"otel.severity_number": "9",
						"otel.severity_text":   "Info",
						"otel.span_id":         traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":       fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":        traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":        "resource-attr-val-1",
						"status":               "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
			},
		},
		{
			name: "new-env-convention",
			args: args{
				ld: func() plog.Logs {
					lrr := testdata.GenerateLogsOneLogRecord()
					lrr.ResourceLogs().At(0).Resource().Attributes().PutStr(conventions127.AttributeDeploymentEnvironmentName, "new_env")
					return lrr
				}(),
				retry: false,
			},
			want: testutil.JSONLogs{
				{
					"message": testutil.JSONLog{
						"@timestamp":                  testdata.TestLogTime.Format(timeFormatString),
						"app":                         "server",
						"dd.span_id":                  fmt.Sprintf("%d", spanIDToUint64(ld.SpanID())),
						"dd.trace_id":                 fmt.Sprintf("%d", traceIDToUint64(ld.TraceID())),
						"deployment.environment.name": "new_env",
						"instance_num":                "1",
						"message":                     ld.Body().AsString(),
						"otel.severity_number":        "9",
						"otel.severity_text":          "Info",
						"otel.span_id":                traceutil.SpanIDToHexOrEmptyString(ld.SpanID()),
						"otel.timestamp":              fmt.Sprintf("%d", testdata.TestLogTime.UnixNano()),
						"otel.trace_id":               traceutil.TraceIDToHexOrEmptyString(ld.TraceID()),
						"resource-attr":               "resource-attr-val-1",
						"status":                      "Info",
					},
					"ddsource": "otlp_log_ingestion",
					"ddtags":   "env:new_env,otel_source:datadog_exporter",
					"service":  "",
					"status":   "Info",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doneChannel := make(chan bool)
			var connectivityCheck sync.Once
			var mockNetworkError sync.Once
			var logsData testutil.JSONLogs
			server := testutil.DatadogLogServerMock(func() (string, http.HandlerFunc) {
				if tt.args.retry {
					return "/api/v2/logs", func(w http.ResponseWriter, r *http.Request) {
						doneConnectivityCheck := false
						connectivityCheck.Do(func() {
							// The logs agent performs a connectivity check upon initialization.
							// This function mocks a successful response for the first request received.
							w.WriteHeader(http.StatusAccepted)
							doneConnectivityCheck = true
						})
						doneMockNetworkError := false
						if !doneConnectivityCheck {
							mockNetworkError.Do(func() {
								w.WriteHeader(http.StatusNotFound)
								doneMockNetworkError = true
							})
						}
						if !doneConnectivityCheck && !doneMockNetworkError {
							jsonLogs := testutil.ProcessLogsAgentRequest(w, r)
							logsData = append(logsData, jsonLogs...)
							doneChannel <- true
						}
					}
				}
				return "/api/v2/logs", func(w http.ResponseWriter, r *http.Request) {
					doneConnectivityCheck := false
					connectivityCheck.Do(func() {
						// The logs agent performs a connectivity check upon initialization.
						// This function mocks a successful response for the first request received.
						w.WriteHeader(http.StatusAccepted)
						doneConnectivityCheck = true
					})
					if !doneConnectivityCheck {
						jsonLogs := testutil.ProcessLogsAgentRequest(w, r)
						logsData = append(logsData, jsonLogs...)
						doneChannel <- true
					}
				}
			})
			defer server.Close()
			cfg := &datadogconfig.Config{
				Logs: datadogconfig.LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: server.URL,
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        1,
				},
			}
			params := exportertest.NewNopSettings(metadata.Type)
			f := NewFactory()
			ctx := context.Background()
			exp, err := f.CreateLogs(ctx, params, cfg)
			require.NoError(t, err)
			require.NoError(t, exp.ConsumeLogs(ctx, tt.args.ld))

			// Wait until `doneChannel` is closed.
			select {
			case <-doneChannel:
				assert.Equal(t, tt.want, logsData)
			case <-time.After(60 * time.Second):
				t.Fail()
			}
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
