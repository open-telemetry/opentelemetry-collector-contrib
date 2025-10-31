// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func TestHttpError_Error(t *testing.T) {
	err := &httpError{
		StatusCode: 500,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte("internal server error"),
		Message:    "request failed with status code 500: internal server error",
	}
	assert.Equal(t, "request failed with status code 500: internal server error", err.Error())
}

func TestNewHTTPLogsExporter(t *testing.T) {
	client := &http.Client{}

	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "with logs endpoint",
			config: &Config{
				Logs: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint: "http://localhost:8080/logs",
					},
				},
			},
			expected: "http://localhost:8080/logs",
		},
		{
			name: "with domain",
			config: &Config{
				Domain: "example.com",
			},
			expected: "https://ingress.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := newHTTPLogsExporter(client, tt.config)
			assert.Equal(t, tt.expected, exporter.Endpoint)
			assert.Equal(t, client, exporter.Client)
		})
	}
}

func TestNewHTTPMetricsExporter(t *testing.T) {
	client := &http.Client{}

	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "with metrics endpoint",
			config: &Config{
				Metrics: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint: "http://localhost:8080/metrics",
					},
				},
			},
			expected: "http://localhost:8080/metrics",
		},
		{
			name: "with domain",
			config: &Config{
				Domain: "example.com",
			},
			expected: "https://ingress.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := newHTTPMetricsExporter(client, tt.config)
			assert.Equal(t, tt.expected, exporter.Endpoint)
			assert.Equal(t, client, exporter.Client)
		})
	}
}

func TestNewHTTPTracesExporter(t *testing.T) {
	client := &http.Client{}

	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "with traces endpoint",
			config: &Config{
				Traces: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint: "http://localhost:8080/traces",
					},
				},
			},
			expected: "http://localhost:8080/traces",
		},
		{
			name: "with domain",
			config: &Config{
				Domain: "example.com",
			},
			expected: "https://ingress.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := newHTTPTracesExporter(client, tt.config)
			assert.Equal(t, tt.expected, exporter.Endpoint)
			assert.Equal(t, client, exporter.Client)
		})
	}
}

func TestHTTPExporter_DoRequest(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		errorType      *httpError
	}{
		{
			name: "successful request",
			path: "/test",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/test", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("success"))
			},
			expectError: false,
		},
		{
			name: "request with trailing slash",
			path: "/test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "server error",
			path: "/test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal error"))
			},
			expectError: true,
			errorType: &httpError{
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			name: "bad request error",
			path: "/test",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("bad request"))
			},
			expectError: true,
			errorType: &httpError{
				StatusCode: http.StatusBadRequest,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			exporter := &httpExporter{
				Client:   server.Client(),
				Endpoint: server.URL,
			}

			body := []byte("test body")
			result, err := exporter.doRequest(t.Context(), body, tt.path)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					var httpErr *httpError
					require.ErrorAs(t, err, &httpErr, "expected httpError")
					assert.Equal(t, tt.errorType.StatusCode, httpErr.StatusCode)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestHTTPLogsExporter_Export(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name: "successful export",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/logs", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				assert.NotEmpty(t, body)

				w.WriteHeader(http.StatusOK)
				resp := plogotlp.NewExportResponse()
				data, _ := resp.MarshalProto()
				_, _ = w.Write(data)
			},
			expectError: false,
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			exporter := &httpLogsExporter{
				httpExporter: httpExporter{
					Client:   server.Client(),
					Endpoint: server.URL,
				},
			}

			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			logRecord := sl.LogRecords().AppendEmpty()
			logRecord.Body().SetStr("test log")

			request := plogotlp.NewExportRequestFromLogs(logs)
			_, err := exporter.Export(t.Context(), request)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPMetricsExporter_Export(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name: "successful export",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/metrics", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				assert.NotEmpty(t, body)

				w.WriteHeader(http.StatusOK)
				resp := pmetricotlp.NewExportResponse()
				data, _ := resp.MarshalProto()
				_, _ = w.Write(data)
			},
			expectError: false,
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			exporter := &httpMetricsExporter{
				httpExporter: httpExporter{
					Client:   server.Client(),
					Endpoint: server.URL,
				},
			}

			metrics := pmetric.NewMetrics()
			rm := metrics.ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			metric := sm.Metrics().AppendEmpty()
			metric.SetName("test_metric")

			request := pmetricotlp.NewExportRequestFromMetrics(metrics)
			_, err := exporter.Export(t.Context(), request)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPTracesExporter_Export(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
	}{
		{
			name: "successful export",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/traces", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				assert.NotEmpty(t, body)

				w.WriteHeader(http.StatusOK)
				resp := ptraceotlp.NewExportResponse()
				data, _ := resp.MarshalProto()
				_, _ = w.Write(data)
			},
			expectError: false,
		},
		{
			name: "server error",
			serverResponse: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			exporter := &httpTracesExporter{
				httpExporter: httpExporter{
					Client:   server.Client(),
					Endpoint: server.URL,
				},
			}

			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			span.SetName("test_span")

			request := ptraceotlp.NewExportRequestFromTraces(traces)
			_, err := exporter.Export(t.Context(), request)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPExporter_WithProxy(t *testing.T) {
	var requestsThroughProxy int

	destServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/logs", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		resp := plogotlp.NewExportResponse()
		data, _ := resp.MarshalProto()
		_, _ = w.Write(data)
	}))
	defer destServer.Close()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsThroughProxy++

		client := &http.Client{}
		proxyReq, _ := http.NewRequest(r.Method, destServer.URL+r.URL.Path, r.Body)
		proxyReq.Header = r.Header

		resp, err := client.Do(proxyReq)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	}))
	defer proxyServer.Close()

	config := &Config{
		Logs: TransportConfig{
			ProxyURL: proxyServer.URL,
		},
	}

	ctx := t.Context()
	host := componenttest.NewNopHost()
	settings := component.TelemetrySettings{}

	client, err := config.Logs.ToHTTPClient(ctx, host, settings)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.Transport, "expected transport to be configured")

	// Create an exporter and make a request
	exporter := &httpLogsExporter{
		httpExporter: httpExporter{
			Client:   client,
			Endpoint: destServer.URL,
		},
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log")

	request := plogotlp.NewExportRequestFromLogs(logs)
	_, err = exporter.Export(ctx, request)
	require.NoError(t, err)

	// Verify request went through proxy
	assert.Equal(t, 1, requestsThroughProxy, "expected request to go through proxy")
}
