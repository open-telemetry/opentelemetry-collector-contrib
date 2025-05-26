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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/metadata"
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
		name           string
		events         []libhoneyevent.LibhoneyEvent
		contentType    string
		expectedStatus int
		wantError      bool
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
			name:           "invalid_content_type",
			events:         []libhoneyevent.LibhoneyEvent{},
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
			wantError:      true,
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

			req := httptest.NewRequest(http.MethodGet, "/1/auth", nil)
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
			req := httptest.NewRequest(tt.method, "/test", nil)
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
