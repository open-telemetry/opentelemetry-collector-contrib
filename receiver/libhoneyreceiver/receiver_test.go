// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"
)

func TestNewLibhoneyReceiver(t *testing.T) {
	defaultCfg := createDefaultConfig()
	httpCfg := defaultCfg.(*Config).HTTP
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid_config",
			config: &Config{
				HTTP: httpCfg,
			},
		},
		{
			name:      "nil_config",
			config:    nil,
			wantError: false,
		},
		{
			name: "config_without_trailing_slashes",
			config: &Config{
				HTTP: &HTTPConfig{
					TracesURLPaths: []string{"/1/events"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := receivertest.NewNopSettings(metadata.Type)
			r, err := newLibhoneyReceiver(tt.config, &set)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, r)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, r)
		})
	}
}

func TestLibhoneyReceiver_Start(t *testing.T) {
	cfg := createDefaultConfig()

	set := receivertest.NewNopSettings(metadata.Type)
	r, err := newLibhoneyReceiver(cfg.(*Config), &set)
	require.NoError(t, err)

	r.registerTraceConsumer(consumertest.NewNop())
	r.registerLogConsumer(consumertest.NewNop())

	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = r.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestLibhoneyReceiver_HandleEvent(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name             string
		events           []libhoneyevent.LibhoneyEvent
		contentType      string
		expectedStatus   int
		expectedResponse []response.ResponseInBatch
		wantError        bool
	}{
		{
			name: "valid_json_event",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Time:             now.Format(time.RFC3339),
					MsgPackTimestamp: &now,
					Data: map[string]any{
						"message": "test event",
					},
					Samplerate: 1,
				},
			},
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
			expectedResponse: []response.ResponseInBatch{
				{Status: http.StatusAccepted},
			},
		},
		{
			name: "valid_msgpack_event",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Time:             now.Format(time.RFC3339),
					MsgPackTimestamp: &now,
					Data: map[string]any{
						"message": "test event",
					},
					Samplerate: 1,
				},
			},
			contentType:    "application/msgpack",
			expectedStatus: http.StatusOK,
		},
		{
			name:             "invalid_content_type",
			events:           []libhoneyevent.LibhoneyEvent{},
			contentType:      "text/plain",
			expectedStatus:   http.StatusUnsupportedMediaType,
			wantError:        true,
			expectedResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig()
			set := receivertest.NewNopSettings(metadata.Type)
			r, err := newLibhoneyReceiver(cfg.(*Config), &set)
			require.NoError(t, err)

			sink := &consumertest.LogsSink{}
			r.registerLogConsumer(sink)

			var body []byte
			switch tt.contentType {
			case "application/json":
				body, err = json.Marshal(tt.events)
			case "application/msgpack":
				body, err = msgpack.Marshal(tt.events)
			default:
				body = []byte("invalid content")
			}
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/1/events/test_dataset", bytes.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			r.handleEvent(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if !tt.wantError {
				assert.Eventually(t, func() bool {
					return sink.LogRecordCount() > 0
				}, time.Second, 10*time.Millisecond)
				assert.Equal(t, tt.contentType, resp.Header.Get("Content-Type"))
			}
			if tt.expectedResponse != nil {
				var actualResponse []response.ResponseInBatch
				err = json.NewDecoder(resp.Body).Decode(&actualResponse)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResponse, actualResponse)
			}
		})
	}
}

func TestLibhoneyReceiver_AuthEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		authAPI        string
		apiKey         string
		mockResponse   *http.Response
		expectedStatus int
	}{
		{
			name:    "valid_auth",
			authAPI: "http://mock-auth-api",
			apiKey:  "test-key",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewBufferString(`{
					"team": {"slug": "test-team"},
					"environment": {"slug": "test-env", "name": "Test Env"}
				}`)),
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.AuthAPI = tt.authAPI
			set := receivertest.NewNopSettings(metadata.Type)
			r, err := newLibhoneyReceiver(cfg, &set)
			require.NoError(t, err)

			// Create test server to mock auth API
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.apiKey, r.Header.Get("x-honeycomb-team"))
				w.WriteHeader(tt.mockResponse.StatusCode)
				_, err := io.Copy(w, tt.mockResponse.Body)
				assert.NoError(t, err, "failed to copy response body")
			}))
			defer ts.Close()

			req := httptest.NewRequest(http.MethodGet, "/1/auth", http.NoBody)
			req.Header.Set("x-honeycomb-team", tt.apiKey)
			w := httptest.NewRecorder()

			r.server = &http.Server{
				Handler: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
					r.handleAuth(resp, req)
				}),
				ReadHeaderTimeout: 3 * time.Second,
			}

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestReadContentType(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
		wantEncoder    bool
	}{
		{
			name:           "valid_json",
			method:         http.MethodPost,
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
			wantEncoder:    true,
		},
		{
			name:           "valid_msgpack",
			method:         http.MethodPost,
			contentType:    "application/msgpack",
			expectedStatus: http.StatusOK,
			wantEncoder:    true,
		},
		{
			name:           "invalid_method",
			method:         http.MethodGet,
			contentType:    "application/json",
			expectedStatus: http.StatusMethodNotAllowed,
			wantEncoder:    false,
		},
		{
			name:           "invalid_content_type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
			wantEncoder:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", http.NoBody)
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			enc, ok := readContentType(w, req)
			assert.Equal(t, tt.wantEncoder, ok)
			if tt.wantEncoder {
				assert.NotNil(t, enc)
			} else {
				assert.Equal(t, tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestLibhoneyReceiver_HandleEvent_WithMetadata(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name             string
		events           []libhoneyevent.LibhoneyEvent
		contentType      string
		headers          map[string]string
		includeMetadata  bool
		expectedMetadata map[string][]string
		expectedStatus   int
	}{
		{
			name: "with_metadata_enabled",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Time:             now.Format(time.RFC3339),
					MsgPackTimestamp: &now,
					Data: map[string]any{
						"message": "test event",
					},
					Samplerate: 1,
				},
			},
			contentType: "application/json",
			headers: map[string]string{
				"x-honeycomb-team":    "test-team",
				"x-honeycomb-dataset": "test-dataset",
				"user-agent":          "test-agent",
			},
			includeMetadata: true,
			expectedMetadata: map[string][]string{
				"x-honeycomb-team":    {"test-team"},
				"x-honeycomb-dataset": {"test-dataset"},
				"user-agent":          {"test-agent"},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "with_metadata_disabled",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Time:             now.Format(time.RFC3339),
					MsgPackTimestamp: &now,
					Data: map[string]any{
						"message": "test event",
					},
					Samplerate: 1,
				},
			},
			contentType: "application/json",
			headers: map[string]string{
				"x-honeycomb-team":    "test-team",
				"x-honeycomb-dataset": "test-dataset",
			},
			includeMetadata:  false,
			expectedMetadata: map[string][]string{},
			expectedStatus:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a custom consumer that captures the context
			var capturedContext context.Context

			customConsumer := &testConsumer{
				logsConsumer:   &consumertest.LogsSink{},
				tracesConsumer: &consumertest.TracesSink{},
				captureContext: func(ctx context.Context, _ plog.Logs, _ ptrace.Traces) {
					capturedContext = ctx
				},
			}

			// Create config with metadata setting
			cfg := createDefaultConfig().(*Config)
			cfg.HTTP.IncludeMetadata = tt.includeMetadata

			set := receivertest.NewNopSettings(metadata.Type)
			r, err := newLibhoneyReceiver(cfg, &set)
			require.NoError(t, err)

			r.registerLogConsumer(customConsumer)
			r.registerTraceConsumer(customConsumer)

			var body []byte
			switch tt.contentType {
			case "application/json":
				body, err = json.Marshal(tt.events)
			case "application/msgpack":
				body, err = msgpack.Marshal(tt.events)
			default:
				body = []byte("invalid content")
			}
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/1/events/test_dataset", bytes.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)

			// Add test headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			// Simulate what confighttp does when IncludeMetadata is enabled
			if tt.includeMetadata {
				// Create metadata from headers
				metadata := make(map[string][]string)
				for key, values := range req.Header {
					metadata[strings.ToLower(key)] = values
				}
				// Add client info to request context
				ctx := client.NewContext(req.Context(), client.Info{
					Metadata: client.NewMetadata(metadata),
				})
				req = req.WithContext(ctx)
			}

			r.handleEvent(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Wait for the consumer to be called
			require.Eventually(t, func() bool {
				return capturedContext != nil
			}, time.Second, 10*time.Millisecond)

			// Check metadata in context
			if tt.includeMetadata {
				info := client.FromContext(capturedContext)
				require.NotNil(t, info.Metadata)

				for key, expectedValues := range tt.expectedMetadata {
					actualValues := info.Metadata.Get(key)
					assert.Equal(t, expectedValues, actualValues, "metadata key: %s", key)
				}
			} else {
				info := client.FromContext(capturedContext)
				// When metadata is disabled, the context should have empty metadata
				// Check that no expected metadata keys are present
				for key := range tt.expectedMetadata {
					actualValues := info.Metadata.Get(key)
					assert.Nil(t, actualValues, "metadata key should not be present: %s", key)
				}
			}
		})
	}
}

// testConsumer is a custom consumer that captures the context and data for testing
type testConsumer struct {
	logsConsumer   *consumertest.LogsSink
	tracesConsumer *consumertest.TracesSink
	captureContext func(context.Context, plog.Logs, ptrace.Traces)
}

func (tc *testConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	tc.captureContext(ctx, logs, ptrace.NewTraces())
	return tc.logsConsumer.ConsumeLogs(ctx, logs)
}

func (tc *testConsumer) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	tc.captureContext(ctx, plog.NewLogs(), traces)
	return tc.tracesConsumer.ConsumeTraces(ctx, traces)
}

func (*testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
