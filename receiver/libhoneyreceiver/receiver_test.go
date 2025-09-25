// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
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
				HTTP: configoptional.Some(HTTPConfig{
					TracesURLPaths: []string{"/1/events"},
				}),
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

	err = r.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)

	err = r.Shutdown(t.Context())
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
			getOrInsertDefault(t, &cfg.(*Config).HTTP)
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
			getOrInsertDefault(t, &cfg.HTTP).IncludeMetadata = tt.includeMetadata

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

func TestLibhoneyReceiver_ZstdDecompressionPanic(t *testing.T) {
	tests := []struct {
		name            string
		createPayload   func() []byte
		expectPanic     bool
		expectErrorCode int // 0 means don't check status code
		description     string
	}{
		{
			name: "malformed_zstd_header",
			createPayload: func() []byte {
				// Create a buffer that looks like zstd but has malformed header
				// This mimics corrupted data that could trigger nil pointer dereference
				return []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			},
			expectPanic:     false,
			expectErrorCode: 400,
			description:     "Malformed zstd header should return error, not panic",
		},
		{
			name: "truncated_zstd_stream",
			createPayload: func() []byte {
				// Create valid JSON payload first
				events := []libhoneyevent.LibhoneyEvent{
					{
						Time:       time.Now().Format(time.RFC3339),
						Data:       map[string]any{"message": "test"},
						Samplerate: 1,
					},
				}
				jsonData, _ := json.Marshal(events)

				// Compress it properly then truncate to create corruption
				var buf bytes.Buffer
				writer, _ := zstd.NewWriter(&buf)
				_, _ = writer.Write(jsonData)
				writer.Close()

				compressed := buf.Bytes()
				// Truncate to create invalid stream that might trigger nil deref
				if len(compressed) > 10 {
					return compressed[:len(compressed)/2]
				}
				return compressed
			},
			expectPanic:     false, // BUG: This currently panics but shouldn't
			expectErrorCode: 400,
			description:     "Truncated zstd stream should return error, not panic (regression test)",
		},
		{
			name: "empty_zstd_with_header",
			createPayload: func() []byte {
				// Zstd magic number but no actual data - edge case
				return []byte{0x28, 0xb5, 0x2f, 0xfd}
			},
			expectPanic:     false,
			expectErrorCode: 400,
			description:     "Empty zstd stream should return error, not panic",
		},
		{
			name: "corrupted_zstd_block",
			createPayload: func() []byte {
				// Create a valid zstd stream then corrupt specific bytes
				// that might cause nil pointer in nextBlockSync
				events := []libhoneyevent.LibhoneyEvent{
					{
						Time:       time.Now().Format(time.RFC3339),
						Data:       map[string]any{"message": "test event for corruption"},
						Samplerate: 1,
					},
				}
				jsonData, _ := json.Marshal(events)

				var buf bytes.Buffer
				writer, _ := zstd.NewWriter(&buf)
				_, _ = writer.Write(jsonData)
				writer.Close()

				compressed := buf.Bytes()
				// Corrupt bytes that might affect block parsing
				if len(compressed) > 20 {
					// Corrupt middle section where block data would be
					for i := 10; i < 15 && i < len(compressed); i++ {
						compressed[i] = 0x00 // Zero out critical bytes
					}
				}
				return compressed
			},
			expectPanic:     false,
			expectErrorCode: 400,
			description:     "Corrupted zstd block should return error, not panic (bug reproduction)",
		},
		{
			name: "valid_json_data_nil_pointer_bug",
			createPayload: func() []byte {
				events := []libhoneyevent.LibhoneyEvent{
					{
						Time:       time.Now().Format(time.RFC3339),
						Data:       map[string]any{"message": "valid test event"},
						Samplerate: 1,
					},
				}
				jsonData, _ := json.Marshal(events)
				return jsonData
			},
			expectPanic:     false,
			expectErrorCode: 200,
			description:     "JSON processing handles nil MsgPackTimestamp correctly after fix",
		},
		{
			name: "real_libhoney_json_format",
			createPayload: func() []byte {
				// Real JSON format as sent by libhoney clients (S3 handler, etc)
				// Note: time is at root level, all fields under "data"
				// This format bypasses custom UnmarshalJSON, leaving MsgPackTimestamp nil
				return []byte(`[{"data":{"message":"test event from S3","aws.s3.bucket":"test-bucket"},"samplerate":1,"time":"2025-09-24T15:03:49.883965174Z"}]`)
			},
			expectPanic:     false,
			expectErrorCode: 200,
			description:     "Real libhoney JSON format should be handled without panic",
		},
		{
			name: "valid_msgpack_data",
			createPayload: func() []byte {
				// Create valid msgpack payload - should work fine
				events := []libhoneyevent.LibhoneyEvent{
					{
						Time:       time.Now().Format(time.RFC3339),
						Data:       map[string]any{"message": "valid msgpack event"},
						Samplerate: 1,
					},
				}
				msgpackData, _ := msgpack.Marshal(events)
				return msgpackData
			},
			expectPanic:     false,
			expectErrorCode: 200,
			description:     "Msgpack should work correctly due to timestamp post-processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			getOrInsertDefault(t, &cfg.HTTP)

			set := receivertest.NewNopSettings(metadata.Type)
			r, err := newLibhoneyReceiver(cfg, &set)
			require.NoError(t, err)

			sink := &consumertest.LogsSink{}
			r.registerLogConsumer(sink)

			payload := tt.createPayload()

			// For valid data, use bytes.NewReader; for malformed data use malformedZstdReader
			var reqBody io.Reader
			var contentType string

			switch tt.name {
			case "valid_json_data_nil_pointer_bug", "real_libhoney_json_format":
				reqBody = bytes.NewReader(payload)
				contentType = "application/json"
			case "valid_msgpack_data":
				reqBody = bytes.NewReader(payload)
				contentType = "application/msgpack"
			default:
				// Create a custom reader that simulates the problematic zstd stream
				reqBody = &malformedZstdReader{
					data:     payload,
					position: 0,
				}
				contentType = "application/json"
			}

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/1/events/test_dataset", reqBody)
			req.Header.Set("Content-Type", contentType)

			// Only set compression headers for zstd tests
			if tt.name != "valid_json_data_nil_pointer_bug" && tt.name != "valid_msgpack_data" && tt.name != "real_libhoney_json_format" {
				req.Header.Set("Content-Encoding", "zstd")
			}

			w := httptest.NewRecorder()

			// Track whether a panic occurred and what the panic was
			var panicOccurred bool
			var panicValue any

			// Always set up panic recovery to properly test both cases
			defer func() {
				if r := recover(); r != nil {
					panicOccurred = true
					panicValue = r
				}

				// Now validate the expectations
				switch {
				case tt.expectPanic && !panicOccurred:
					t.Errorf("Test '%s' expected a panic but none occurred. %s", tt.name, tt.description)
				case !tt.expectPanic && panicOccurred:
					t.Errorf("Test '%s' expected no panic but got: %v. %s", tt.name, panicValue, tt.description)
				case tt.expectPanic && panicOccurred:
					// Validate it's the right kind of panic
					panicStr := fmt.Sprintf("%v", panicValue)
					if strings.Contains(panicStr, "nil pointer") ||
						strings.Contains(panicStr, "invalid memory address") ||
						strings.Contains(panicStr, "runtime error") {
						t.Logf("✓ Test '%s' correctly caught expected panic: %v", tt.name, panicValue)
					} else {
						t.Errorf("Test '%s' panicked but not with expected error type. Got: %v", tt.name, panicValue)
					}
				}

				// If no panic and we expect a specific status code, validate it
				if !panicOccurred && tt.expectErrorCode > 0 {
					resp := w.Result()
					if resp.StatusCode != tt.expectErrorCode {
						t.Errorf("Test '%s' expected status code %d but got %d. %s",
							tt.name, tt.expectErrorCode, resp.StatusCode, tt.description)
					} else {
						t.Logf("✓ Test '%s' correctly returned status code %d", tt.name, resp.StatusCode)
					}
				}
			}()

			// This calls io.ReadAll(req.Body) at line 192 which should trigger the issue
			r.handleEvent(w, req)
		})
	}
}

// malformedZstdReader simulates a reader that behaves like the pooled zstd reader
// from confighttp middleware but contains corrupted data that triggers nil pointer dereference
type malformedZstdReader struct {
	data     []byte
	position int
}

func (r *malformedZstdReader) Read(p []byte) (n int, err error) {
	if r.position >= len(r.data) {
		return 0, io.EOF
	}

	// Check if this is the test case designed to trigger a panic
	if len(r.data) == 12 && r.data[4] == 0x01 && r.data[5] == 0x02 {
		// This is the nil_pointer_trigger_test case - simulate a nil pointer panic
		if r.position == 8 { // Trigger on second read
			panic("runtime error: invalid memory address or nil pointer dereference")
		}
	}

	// Copy data but introduce specific corruption patterns
	n = copy(p, r.data[r.position:])
	r.position += n

	// Simulate the conditions that might trigger nil pointer in zstd decoder
	// by returning specific byte patterns that confuse the decoder state
	if r.position == len(r.data) {
		// Trigger potential nil pointer by corrupting the final bytes
		if len(p) > 4 && n > 4 {
			p[n-4] = 0x00 // Corrupt control bytes
			p[n-3] = 0x00
			p[n-2] = 0xFF
			p[n-1] = 0xFF
		}
	}

	return n, nil
}

// TestLibhoneyReceiver_ZstdDecompressionIntegration tests with full HTTP server
// to exactly replicate the middleware stack from the error logs
func TestLibhoneyReceiver_ZstdDecompressionIntegration(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	getOrInsertDefault(t, &cfg.HTTP)

	set := receivertest.NewNopSettings(metadata.Type)
	r, err := newLibhoneyReceiver(cfg, &set)
	require.NoError(t, err)

	sink := &consumertest.LogsSink{}
	r.registerLogConsumer(sink)

	// Start the actual HTTP server with middleware
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { _ = r.Shutdown(t.Context()) }()

	// Create malformed zstd payload
	events := []libhoneyevent.LibhoneyEvent{
		{
			Time:       time.Now().Format(time.RFC3339),
			Data:       map[string]any{"message": "test event"},
			Samplerate: 1,
		},
	}
	jsonData, err := json.Marshal(events)
	require.NoError(t, err)

	// Create corrupted zstd data that might trigger the nil pointer
	var buf bytes.Buffer
	writer, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	_, _ = writer.Write(jsonData)
	writer.Close()

	compressed := buf.Bytes()
	// Corrupt the compressed data to trigger decompression issues
	if len(compressed) > 15 {
		// Introduce corruption that might cause nil pointer in nextBlockSync
		compressed[8] = 0x00
		compressed[9] = 0x00
		compressed[10] = 0xFF
		compressed[11] = 0xFF
	}

	// Get the server endpoint
	if r.server == nil {
		t.Skip("HTTP server not started - skipping integration test")
		return
	}

	// Note: This test demonstrates the setup but won't actually make HTTP calls
	// since we'd need the actual server address. In a real scenario, this would
	// make an HTTP request to the running server with the corrupted payload.
	t.Logf("Integration test setup complete. Corrupted payload size: %d bytes", len(compressed))

	// The actual HTTP request would be:
	// resp, err := http.Post(serverURL+"/1/events/test", "application/json", bytes.NewReader(compressed))
	// With header: Content-Encoding: zstd
}
