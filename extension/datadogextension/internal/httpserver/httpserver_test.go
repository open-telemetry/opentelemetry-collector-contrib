// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

func TestServerStart(t *testing.T) {
	tests := []struct {
		name         string
		setupServer  func() (*Server, *observer.ObservedLogs)
		expectedLogs []string
	}{
		{
			name: "Start server successfully",
			setupServer: func() (*Server, *observer.ObservedLogs) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				s := NewServer(
					logger,
					&mockSerializer{},
					&Config{
						ServerConfig: confighttp.ServerConfig{
							Endpoint: DefaultServerEndpoint,
						},
						Path: "/metadata",
					},
					"test-hostname",
					"test-uuid",
					payload.OtelCollector{},
				)
				return s, logs
			},
			expectedLogs: []string{fmt.Sprintf("HTTP Server started at %s%s", DefaultServerEndpoint, "/metadata")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, logs := tt.setupServer()
			s.Start()

			// Verify the logs
			for _, expectedLog := range tt.expectedLogs {
				found := false
				for _, log := range logs.All() {
					if log.Message == expectedLog {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected log message not found: %s", expectedLog)
			}

			// Stop the server
			s.Stop(context.Background())
		})
	}
}

func TestPrepareAndSendFleetAutomationPayloads(t *testing.T) {
	tests := []struct {
		name               string
		setupTest          func() (*zap.Logger, *observer.ObservedLogs, agentcomponents.SerializerWithForwarder)
		expectedError      string
		expectedLogs       []string
		serverResponseCode int
		serverResponse     string
	}{
		{
			name: "Successful payload preparation and sending",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, agentcomponents.SerializerWithForwarder) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				serializer := &mockSerializer{
					sendMetadataFunc: func(any) error {
						return nil
					},
				}
				err := serializer.Start()
				if err != nil {
					t.Fatalf("Failed to start serializer: %v", err)
				}
				return logger, logs, serializer
			},
			expectedError:      "",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusOK,
			serverResponse:     `{"status": "ok"}`,
		},
		{
			name: "Failed to get health check status",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, agentcomponents.SerializerWithForwarder) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				serializer := &mockSerializer{
					sendMetadataFunc: func(any) error {
						return nil
					},
				}
				err := serializer.Start()
				if err != nil {
					t.Fatalf("Failed to start serializer: %v", err)
				}
				return logger, logs, serializer
			},
			expectedError:      "",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusInternalServerError,
			serverResponse:     `Internal Server Error`,
		},
		{
			name: "Failed to send payload",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, agentcomponents.SerializerWithForwarder) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				serializer := &mockSerializer{
					sendMetadataFunc: func(pl any) error {
						if _, ok := pl.(*payload.OtelCollectorPayload); ok {
							return errors.New("failed to send payload")
						}
						return nil
					},
				}
				err := serializer.Start()
				if err != nil {
					t.Fatalf("Failed to start serializer: %v", err)
				}
				return logger, logs, serializer
			},
			expectedError:      "failed to send payload to Datadog: failed to send payload",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusInternalServerError,
			serverResponse:     `Internal Server Error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs, serializer := tt.setupTest()
			s := NewServer(
				logger,
				serializer,
				&Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: DefaultServerEndpoint,
					},
					Path: "/metadata",
				},
				"test-hostname",
				"test-uuid",
				payload.OtelCollector{},
			)
			ocPayload, err := s.SendPayload()
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ocPayload)
			}
			for _, expectedLog := range tt.expectedLogs {
				found := false
				for _, log := range logs.All() {
					if log.Message == expectedLog {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected log message not found: %s", expectedLog)
			}
		})
	}
}

const successfulInstanceResponse = `{
  "hostname": "test-hostname",
  "timestamp": 1741003200000000000,
  "otel_collector": {
    "host_key": "",
    "hostname": "",
    "hostname_source": "",
    "collector_id": "",
    "collector_version": "",
    "config_site": "",
    "api_key_uuid": "",
    "full_components": [],
    "active_components": [],
    "build_info": {
      "command": "",
      "description": "",
      "version": ""
    },
    "full_configuration": "",
    "health_status": ""
  },
  "uuid": "test-uuid"
}`

func TestHandleMetadata(t *testing.T) {
	mockTime := time.Date(2025, time.March, 3, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time {
		return mockTime
	}
	defer func() {
		nowFunc = time.Now
	}()
	tests := []struct {
		name           string
		setupTest      func() (*zap.Logger, agentcomponents.SerializerWithForwarder)
		hostnameSource string
		expectedCode   int
		expectedBody   string
	}{
		{
			name: "Successful metadata handling",
			setupTest: func() (*zap.Logger, agentcomponents.SerializerWithForwarder) {
				core, _ := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				serializer := &mockSerializer{
					sendMetadataFunc: func(any) error {
						return nil
					},
				}
				err := serializer.Start()
				if err != nil {
					t.Fatalf("Failed to start serializer: %v", err)
				}
				return logger, serializer
			},
			hostnameSource: "config",
			expectedCode:   http.StatusOK,
			expectedBody:   successfulInstanceResponse,
		},
		{
			name: "Failed metadata handling - serializer error",
			setupTest: func() (*zap.Logger, agentcomponents.SerializerWithForwarder) {
				core, _ := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				serializer := &mockSerializer{
					sendMetadataFunc: func(any) error {
						return errors.New("failed to send metadata")
					},
				}
				err := serializer.Start()
				if err != nil {
					t.Fatalf("Failed to start serializer: %v", err)
				}
				return logger, serializer
			},
			hostnameSource: "config",
			expectedCode:   http.StatusInternalServerError,
			expectedBody:   "Failed to prepare and send fleet automation payload\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, serializer := tt.setupTest()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/metadata", nil)
			r.Header.Set("Content-Type", "application/json")
			srv := &Server{
				logger:     logger,
				serializer: serializer,
				payload: &payload.OtelCollectorPayload{
					Hostname: "test-hostname",
					UUID:     "test-uuid",
					Metadata: payload.OtelCollector{
						FullComponents:   []payload.CollectorModule{},
						ActiveComponents: []payload.ServiceComponent{},
					},
				},
			}
			srv.HandleMetadata(w, r)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestHandleMetadataConcurrency(t *testing.T) {
	mockTime := time.Date(2025, time.March, 3, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time {
		return mockTime
	}
	defer func() {
		nowFunc = time.Now
	}()

	// Track the number of concurrent calls to SendMetadata
	var callCount int32
	var maxConcurrentCalls int32
	var currentConcurrentCalls int32

	core, _ := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	serializer := &mockSerializer{
		sendMetadataFunc: func(any) error {
			// Increment current concurrent calls
			current := atomic.AddInt32(&currentConcurrentCalls, 1)

			// Update max if this is higher
			for {
				maxVal := atomic.LoadInt32(&maxConcurrentCalls)
				if current <= maxVal || atomic.CompareAndSwapInt32(&maxConcurrentCalls, maxVal, current) {
					break
				}
			}

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			// Decrement current concurrent calls and increment total call count
			atomic.AddInt32(&currentConcurrentCalls, -1)
			atomic.AddInt32(&callCount, 1)

			return nil
		},
	}
	err := serializer.Start()
	require.NoError(t, err)

	srv := &Server{
		logger:     logger,
		serializer: serializer,
		payload: &payload.OtelCollectorPayload{
			Metadata: payload.OtelCollector{
				FullComponents:   []payload.CollectorModule{},
				ActiveComponents: []payload.ServiceComponent{},
			},
			Hostname: "test-hostname",
			UUID:     "test-uuid",
		},
	}

	// Test concurrent requests
	const numRequests = 10
	var wg sync.WaitGroup
	responses := make([]*httptest.ResponseRecorder, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/metadata", nil)
			r.Header.Set("Content-Type", "application/json")
			responses[index] = w
			srv.HandleMetadata(w, r)
		}(i)
	}

	wg.Wait()

	// Verify all requests completed successfully
	for i, resp := range responses {
		assert.Equal(t, http.StatusOK, resp.Code, "Request %d should succeed", i)
		assert.NotEmpty(t, resp.Body.String(), "Request %d should have response body", i)
	}

	// Verify the expected number of serializer calls were made
	assert.Equal(t, int32(numRequests), atomic.LoadInt32(&callCount), "Should have made %d serializer calls", numRequests)

	// Log the maximum concurrent calls for debugging
	t.Logf("Maximum concurrent calls to SendMetadata: %d", atomic.LoadInt32(&maxConcurrentCalls))

	// Note: This test verifies that concurrent calls don't crash or deadlock,
	// but it doesn't guarantee thread safety of the underlying serializer.
	// That would need to be tested at the serializer level.
}

func TestServerStop(t *testing.T) {
	tests := []struct {
		name             string
		setupServer      func() (*Server, *observer.ObservedLogs)
		contextSetup     func() (context.Context, context.CancelFunc)
		expectedLogs     []string
		expectTimeout    bool
		simulateSlowStop bool
	}{
		{
			name: "Stop with nil server - should be no-op",
			setupServer: func() (*Server, *observer.ObservedLogs) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return &Server{
					logger: logger,
					server: nil, // nil server
				}, logs
			},
			contextSetup: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 100*time.Millisecond)
			},
			expectedLogs:  []string{},
			expectTimeout: false,
		},
		{
			name: "Successful shutdown completes before context timeout",
			setupServer: func() (*Server, *observer.ObservedLogs) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)

				// Create a test server that will shut down quickly
				mux := http.NewServeMux()
				mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				//nolint: gosec // G112: Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server
				server := &http.Server{
					Addr:    "127.0.0.1:0", // Use any available port
					Handler: mux,
				}

				return &Server{
					logger: logger,
					server: server,
				}, logs
			},
			contextSetup: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Second)
			},
			expectedLogs:  []string{},
			expectTimeout: false,
		},
		{
			name: "Context cancelled before shutdown completes",
			setupServer: func() (*Server, *observer.ObservedLogs) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)

				// Create a test server
				mux := http.NewServeMux()
				mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				//nolint: gosec // G112: Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server
				server := &http.Server{
					Addr:    "127.0.0.1:0",
					Handler: mux,
				}

				return &Server{
					logger: logger,
					server: server,
				}, logs
			},
			contextSetup: func() (context.Context, context.CancelFunc) {
				// Create a context that will be cancelled very quickly
				ctx, cancel := context.WithCancel(context.Background())
				// Cancel the context after a short delay to simulate timeout
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			expectedLogs:     []string{"Context cancelled while waiting for server shutdown"},
			expectTimeout:    true,
			simulateSlowStop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, logs := tt.setupServer()
			ctx, cancel := tt.contextSetup()
			defer cancel()

			// If we have a server, start it briefly to make shutdown more realistic
			if srv.server != nil && srv.server.Addr != "" {
				// Start the server in a goroutine
				go func() {
					if err := srv.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						t.Logf("Unexpected server error: %v", err)
					}
				}()
				// Give the server a moment to start
				time.Sleep(10 * time.Millisecond)
			}

			// Measure how long Stop takes
			start := time.Now()
			srv.Stop(ctx)
			duration := time.Since(start)

			// Verify expected logs
			for _, expectedLog := range tt.expectedLogs {
				found := false
				for _, log := range logs.All() {
					if log.Message == expectedLog {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected log message not found: %s", expectedLog)
			}

			// If we expect timeout, verify it happened reasonably quickly
			// (context cancellation should not wait for full server shutdown)
			if tt.expectTimeout {
				assert.Less(t, duration, 500*time.Millisecond, "Stop should return quickly when context is cancelled")
			}

			t.Logf("Stop duration: %v", duration)
		})
	}
}

func TestServerStopChannelBehavior(t *testing.T) {
	// This test specifically focuses on channel closure behavior
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Create a custom server implementation to test channel behavior
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	//nolint: gosec // G112: Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	srv := &Server{
		logger: logger,
		server: server,
	}

	// Start the server
	go func() {
		if err := srv.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Unexpected server error: %v", err)
		}
	}()
	time.Sleep(10 * time.Millisecond) // Let server start

	t.Run("Channel closed on successful shutdown", func(t *testing.T) {
		ctx := context.Background()

		// Use a channel to detect when Stop completes
		done := make(chan struct{})
		go func() {
			srv.Stop(ctx)
			close(done)
		}()

		// Wait for Stop to complete
		select {
		case <-done:
			// Success - Stop completed
		case <-time.After(1 * time.Second):
			t.Fatal("Stop did not complete within timeout")
		}

		// Verify no error logs for successful shutdown
		errorLogs := 0
		for _, log := range logs.All() {
			if log.Level == zapcore.ErrorLevel {
				errorLogs++
			}
		}
		assert.Equal(t, 0, errorLogs, "Should not have error logs for successful shutdown")
	})
}

func TestServerStopConcurrency(t *testing.T) {
	// Test that multiple calls to Stop don't cause issues
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	//nolint: gosec // G112: Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	srv := &Server{
		logger: logger,
		server: server,
	}

	// Start the server
	go func() {
		if err := srv.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Unexpected server error: %v", err)
		}
	}()
	time.Sleep(10 * time.Millisecond)

	// Call Stop concurrently multiple times
	const numStops = 5
	var wg sync.WaitGroup

	for i := 0; i < numStops; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			srv.Stop(ctx)
		}(i)
	}

	// Wait for all Stop calls to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All Stop calls completed successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Concurrent Stop calls did not complete within timeout")
	}

	// Verify we don't have excessive error logs (some might be expected due to concurrent shutdowns)
	errorLogs := 0
	for _, log := range logs.All() {
		if log.Level == zapcore.ErrorLevel {
			errorLogs++
			t.Logf("Error log: %s", log.Message)
		}
	}

	// We might have some errors due to concurrent shutdowns, but shouldn't be excessive
	assert.LessOrEqual(t, errorLogs, numStops, "Should not have excessive error logs")

	t.Logf("Completed %d concurrent Stop calls with %d error logs", numStops, errorLogs)
}

func TestServer_SendPayload(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:0",
		},
		Path: "/test",
	}
	pl := payload.OtelCollector{}           // or fill as needed
	serializer := &mockSerializer{state: 1} // 1 == defaultforwarder.Started
	called := false
	serializer.sendMetadataFunc = func(any) error {
		called = true
		return nil
	}

	server := NewServer(logger, serializer, config, "host", "uuid", pl)

	result, err := server.SendPayload()
	assert.NoError(t, err)
	assert.True(t, called)
	// Check that result is a *payload.OtelCollectorPayload
	oc, ok := result.(*payload.OtelCollectorPayload)
	assert.True(t, ok)
	assert.Equal(t, "host", oc.Hostname)
	assert.Equal(t, "uuid", oc.UUID)
}

func TestServer_SendPayload_ForwarderNotStarted(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:0",
		},
		Path: "/test",
	}
	pl := payload.OtelCollector{}
	serializer := &mockSerializer{state: 0} // 0 != defaultforwarder.Started

	server := NewServer(logger, serializer, config, "host", "uuid", pl)

	result, err := server.SendPayload()
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "forwarder is not started")
}

func TestHandleMetadata_JSONMarshalError(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	serializer := &mockSerializer{
		sendMetadataFunc: func(any) error {
			return nil
		},
	}
	err := serializer.Start()
	if err != nil {
		t.Fatalf("Failed to start serializer: %v", err)
	}

	srv := &Server{
		logger:     logger,
		serializer: serializer,
		payload:    &mockJSONErrorPayload{},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/metadata", nil)
	r.Header.Set("Content-Type", "application/json")

	srv.HandleMetadata(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Failed to marshal collector payload\n", w.Body.String())

	found := false
	for _, log := range logs.All() {
		if log.Message == "Failed to marshal collector payload for local http response" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected error log for marshal failure")
}
