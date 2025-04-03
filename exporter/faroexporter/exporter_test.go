// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	faro "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter/internal/metadata"
)

func TestExporter_BaseTest(t *testing.T) {
	tc := []struct {
		name   string
		config *Config
	}{
		{
			name: "gzip",
			config: func() *Config {
				return createDefaultConfig().(*Config)
			}(),
		},
		{
			name: "deflate",
			config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.Compression = configcompression.TypeDeflate
				return cfg
			}(),
		},
		{
			name: "no compression",
			config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ClientConfig.CompressionParams = configcompression.CompressionParams{
					Level: configcompression.Level(0),
				}
				return cfg
			}(),
		},
	}

	server := createServer(t)
	defer server.Close()

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			cfg := c.config
			cfg.Endpoint = server.URL

			set := exportertest.NewNopSettings(metadata.Type)
			exp, err := newExporter(cfg, set)
			require.NoError(t, err)
			require.NotNil(t, exp)

			ctx := context.Background()
			err = exp.start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			td := createTestTraces()
			err = exp.ConsumeTraces(ctx, td)
			assert.NoError(t, err)

			ld := createTestLogs()
			err = exp.ConsumeLogs(ctx, ld)
			assert.NoError(t, err)
		})
	}
}

func createServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("User-Agent"))

		var reader io.Reader
		var err error
		encoding := r.Header.Get("Content-Encoding")
		switch encoding {
		case "gzip":
			reader, err = gzip.NewReader(r.Body)
			assert.NoError(t, err)
		case "deflate":
			reader, err = zlib.NewReader(r.Body)
			assert.NoError(t, err)
		default:
			reader = r.Body
		}

		bodyBytes, err := io.ReadAll(reader)
		assert.NoError(t, err)
		defer r.Body.Close()

		payload := faro.Payload{}
		err = json.Unmarshal(bodyBytes, &payload)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusAccepted)
	}))
}

func TestExporter_ErrorCases(t *testing.T) {
	testCases := []struct {
		name               string
		statusCode         int
		expectedPermanent  bool
		expectedRetryable  bool
		expectedThrottled  bool
		setRetryAfterValue bool
		responseBody       string
		checkResponseBody  bool
	}{
		{
			name:              "permanent error",
			statusCode:        http.StatusBadRequest,
			expectedPermanent: true,
			responseBody:      "Bad request: invalid format",
			checkResponseBody: true,
		},
		{
			name:              "retryable error",
			statusCode:        http.StatusInternalServerError,
			expectedRetryable: true,
			responseBody:      "Internal server error: try again later",
			checkResponseBody: true,
		},
		{
			name:               "throttled with retry-after",
			statusCode:         http.StatusTooManyRequests,
			expectedRetryable:  true,
			expectedThrottled:  true,
			setRetryAfterValue: true,
			responseBody:       "Rate limit exceeded",
			checkResponseBody:  true,
		},
		{
			name:              "service unavailable",
			statusCode:        http.StatusServiceUnavailable,
			expectedRetryable: true,
			expectedThrottled: true,
			responseBody:      "Service temporarily unavailable",
			checkResponseBody: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.setRetryAfterValue {
					w.Header().Set("Retry-After", "30")
				}
				w.WriteHeader(tc.statusCode)
				if tc.responseBody != "" {
					_, _ = w.Write([]byte(tc.responseBody))
				}
			}))
			defer server.Close()

			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = server.URL

			set := exportertest.NewNopSettings(metadata.Type)
			exp, err := newExporter(cfg, set)
			require.NoError(t, err)
			require.NotNil(t, exp)

			ctx := context.Background()
			err = exp.start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			td := createTestTraces()
			err = exp.ConsumeTraces(ctx, td)

			if tc.checkResponseBody {
				errMsg := err.Error()
				assert.Contains(t, errMsg, tc.responseBody, "Error message should contain response body content")
			}

			switch {
			case tc.expectedPermanent:
				assert.True(t, consumererror.IsPermanent(err), "Expected permanent error")
			case tc.expectedRetryable:
				if tc.expectedThrottled {
					errMsg := err.Error()
					assert.Contains(t, errMsg, "Throttle", "Expected throttling error message")
					if tc.setRetryAfterValue {
						assert.Contains(t, errMsg, "30s", "Expected retry after duration in error")
					}
				} else {
					assert.Error(t, err, "Expected retryable error")
				}
			}
		})
	}
}

func TestExportContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = server.URL
	set := exportertest.NewNopSettings(metadata.Type)
	exp, err := newExporter(cfg, set)
	require.NoError(t, err)
	require.NotNil(t, exp)

	ctx, cancel := context.WithCancel(context.Background())
	err = exp.start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	cancel()

	td := createTestTraces()
	err = exp.ConsumeTraces(ctx, td)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to make an HTTP request: Post \""+server.URL+"\": context canceled")
}

func createTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()

	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutStr("service.version", "1.0.0")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Attributes().PutStr("test.attribute", "test-value")

	return traces
}

func createTestLogs() plog.Logs {
	logs := plog.NewLogs()
	rs := logs.ResourceLogs().AppendEmpty()

	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutStr("service.version", "1.0.0")

	lr := rs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.SetSeverityNumber(plog.SeverityNumberInfo)
	lr.SetSeverityText("info")
	lr.Body().SetStr("kind=event message=This is a test log message")
	return logs
}
