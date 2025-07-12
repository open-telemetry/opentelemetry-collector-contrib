// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal/metadata"
)

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "build exporter",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://localhost:8080",
				},
				Token:   "test-token",
				Metrics: SignalConfig{Datasource: "metrics_test"},
				Traces:  SignalConfig{Datasource: "traces_test"},
				Logs:    SignalConfig{Datasource: "logs_test"},
				Wait:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := newExporter(tt.config, exportertest.NewNopSettings(metadata.Type))
			assert.NotNil(t, exp)
		})
	}
}

func TestExportTraces(t *testing.T) {
	type args struct {
		traces ptrace.Traces
		config Config
	}
	type want struct {
		requestQuery   string
		requestBody    string
		responseStatus int
		err            error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "export without traces",
			args: args{
				traces: func() ptrace.Traces {
					traces := ptrace.NewTraces()
					rs := traces.ResourceSpans().AppendEmpty()
					rs.ScopeSpans().AppendEmpty()
					return traces
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Traces:       SignalConfig{Datasource: "traces_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=traces_test",
				requestBody:    "",
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with full trace",
			args: args{
				traces: func() ptrace.Traces {
					traces := ptrace.NewTraces()
					rs := traces.ResourceSpans().AppendEmpty()
					rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rs.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					ss := rs.ScopeSpans().AppendEmpty()
					ss.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := ss.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					span := ss.Spans().AppendEmpty()
					span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					span.SetParentSpanID(pcommon.SpanID([8]byte{9, 10, 11, 12, 13, 14, 15, 16}))
					span.SetName("test-span")
					span.SetKind(ptrace.SpanKindServer)
					span.SetStartTimestamp(pcommon.Timestamp(1719158400000000000)) // 2024-06-23T16:00:00Z
					span.SetEndTimestamp(pcommon.Timestamp(1719158401000000000))   // 2024-06-23T16:00:01Z
					span.Status().SetCode(ptrace.StatusCodeOk)
					span.Status().SetMessage("success")
					span.Attributes().PutStr("http.method", "GET")
					span.Attributes().PutStr("http.url", "/api/users")
					span.Attributes().PutStr("user.id", "12345")

					// Add span event
					event := span.Events().AppendEmpty()
					event.SetName("exception")
					event.SetTimestamp(pcommon.Timestamp(1719158400500000000)) // 2024-06-23T16:00:00.5Z
					event.Attributes().PutStr("exception.type", "RuntimeException")
					event.Attributes().PutStr("exception.message", "Something went wrong")

					// Add span link
					link := span.Links().AppendEmpty()
					link.SetTraceID(pcommon.TraceID([16]byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}))
					link.SetSpanID(pcommon.SpanID([8]byte{17, 18, 19, 20, 21, 22, 23, 24}))
					link.TraceState().FromRaw("sampled=true")
					link.Attributes().PutStr("link.type", "child")

					return traces
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Traces:       SignalConfig{Datasource: "traces_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=traces_test",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_name":"test-scope","scope_version":"1.0.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","parent_span_id":"090a0b0c0d0e0f10","trace_state":"","trace_flags":0,"span_name":"test-span","span_kind":"Server","span_attributes":{"http.method":"GET","http.url":"/api/users","user.id":"12345"},"start_time":"2024-06-23T16:00:00Z","end_time":"2024-06-23T16:00:01Z","duration":1000000000,"status_code":"Ok","status_message":"success","events_timestamp":["2024-06-23T16:00:00.5Z"],"events_name":["exception"],"events_attributes":[{"exception.type":"RuntimeException","exception.message":"Something went wrong"}],"links_trace_id":["1112131415161718191a1b1c1d1e1f20"],"links_span_id":["1112131415161718"],"links_trace_state":["sampled=true"],"links_attributes":[{"link.type":"child"}]}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v0/events", r.URL.Path)
				assert.Equal(t, tt.want.requestQuery, r.URL.RawQuery)
				assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer "+string(tt.args.config.Token), r.Header.Get("Authorization"))
				gotBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.JSONEq(t, tt.want.requestBody, string(gotBody))

				w.WriteHeader(tt.want.responseStatus)
			}))
			defer server.Close()

			tt.args.config.ClientConfig.Endpoint = server.URL

			exp := newExporter(&tt.args.config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			err := exp.pushTraces(context.Background(), tt.args.traces)
			if tt.want.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportLogs(t *testing.T) {
	type args struct {
		logs   plog.Logs
		config Config
	}
	type want struct {
		requestQuery   string
		requestBody    string
		responseStatus int
		err            error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "export without logs",
			args: args{
				logs: func() plog.Logs {
					logs := plog.NewLogs()
					rl := logs.ResourceLogs().AppendEmpty()
					rl.ScopeLogs().AppendEmpty()
					return logs
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Logs:         SignalConfig{Datasource: "logs_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=logs_test",
				requestBody:    "",
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
		{
			name: "export with full log",
			args: args{
				logs: func() plog.Logs {
					logs := plog.NewLogs()
					rl := logs.ResourceLogs().AppendEmpty()
					rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					resource := rl.Resource()
					resource.Attributes().PutStr("service.name", "test-service")
					resource.Attributes().PutStr("environment", "production")

					sl := rl.ScopeLogs().AppendEmpty()
					sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.20.0")
					scope := sl.Scope()
					scope.SetName("test-scope")
					scope.SetVersion("1.0.0")
					scope.Attributes().PutStr("telemetry.sdk.name", "opentelemetry")

					log := sl.LogRecords().AppendEmpty()
					log.Body().SetStr("User login attempt")
					log.Attributes().PutStr("http.method", "POST")
					log.Attributes().PutStr("http.url", "/api/login")
					log.Attributes().PutStr("user.id", "12345")
					log.SetTimestamp(pcommon.Timestamp(1719158401000000000)) // 2024-06-23T16:00:01Z
					log.SetSeverityText("INFO")
					log.SetSeverityNumber(plog.SeverityNumberInfo)
					log.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
					log.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
					log.SetFlags(plog.LogRecordFlags(1))
					return logs
				}(),
				config: Config{
					ClientConfig: confighttp.ClientConfig{},
					Token:        "test-token",
					Logs:         SignalConfig{Datasource: "logs_test"},
					Wait:         false,
				},
			},
			want: want{
				requestQuery:   "name=logs_test",
				requestBody:    `{"resource_schema_url":"https://opentelemetry.io/schemas/1.20.0","resource_attributes":{"service.name":"test-service","environment":"production"},"service_name":"test-service","scope_name":"test-scope","scope_version":"1.0.0","scope_schema_url":"https://opentelemetry.io/schemas/1.20.0","scope_attributes":{"telemetry.sdk.name":"opentelemetry"},"body":"User login attempt","log_attributes":{"http.method":"POST","http.url":"/api/login","user.id":"12345"},"timestamp":"2024-06-23T16:00:01Z","severity_text":"INFO","severity_number":9,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","flags":1}`,
				responseStatus: http.StatusOK,
				err:            nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v0/events", r.URL.Path)
				assert.Equal(t, tt.want.requestQuery, r.URL.RawQuery)
				assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer "+string(tt.args.config.Token), r.Header.Get("Authorization"))
				gotBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.JSONEq(t, tt.want.requestBody, string(gotBody))

				w.WriteHeader(tt.want.responseStatus)
			}))
			defer server.Close()

			tt.args.config.ClientConfig.Endpoint = server.URL

			exp := newExporter(&tt.args.config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			err := exp.pushLogs(context.Background(), tt.args.logs)
			if tt.want.err != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		headers        map[string]string
		wantErr        bool
	}{
		{
			name:           "success",
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "throttled",
			responseStatus: http.StatusTooManyRequests,
			headers:        map[string]string{"Retry-After": "30"},
			wantErr:        true,
		},
		{
			name:           "service unavailable",
			responseStatus: http.StatusServiceUnavailable,
			wantErr:        true,
		},
		{
			name:           "permanent error",
			responseStatus: http.StatusBadRequest,
			responseBody:   "invalid request",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, err := w.Write([]byte(tt.responseBody))
					assert.NoError(t, err)
				}
			}))
			defer server.Close()

			config := &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: server.URL,
				},
				Token:   "test-token",
				Metrics: SignalConfig{Datasource: "metrics_test"},
				Traces:  SignalConfig{Datasource: "traces_test"},
				Logs:    SignalConfig{Datasource: "logs_test"},
			}

			exp := newExporter(config, exportertest.NewNopSettings(metadata.Type))
			require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))

			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			lr := sl.LogRecords().AppendEmpty()
			lr.Body().SetStr("test-log")
			err := exp.pushLogs(context.Background(), logs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
