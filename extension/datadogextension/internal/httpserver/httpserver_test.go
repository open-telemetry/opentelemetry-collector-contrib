// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
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
				c := confmap.New()
				s := NewServer(
					logger,
					&Config{
						ServerConfig: confighttp.ServerConfig{
							Endpoint: DefaultServerEndpoint,
						},
						Path: "/metadata",
					},
					&mockSerializer{},
					&mockForwarder{},
					"test-hostname-source",
					"test-hostname",
					"test-uuid",
					&map[string]any{},
					&service.ModuleInfos{},
					c,
					&payload.OtelCollector{},
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
			s.Stop()
		})
	}
}

func TestPrepareAndSendFleetAutomationPayloads(t *testing.T) {
	tests := []struct {
		name               string
		setupTest          func() (*zap.Logger, *observer.ObservedLogs, serializer.MetricSerializer, defaultForwarderInterface)
		expectedError      string
		expectedLogs       []string
		serverResponseCode int
		serverResponse     string
	}{
		{
			name: "Successful payload preparation and sending",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, serializer.MetricSerializer, defaultForwarderInterface) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return logger, logs, &mockSerializer{
						sendMetadataFunc: func(any) error {
							return nil
						},
					}, &mockForwarder{
						state: 1,
					}
			},
			expectedError:      "",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusOK,
			serverResponse:     `{"status": "ok"}`,
		},
		{
			name: "Failed to get health check status",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, serializer.MetricSerializer, defaultForwarderInterface) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return logger, logs, &mockSerializer{
						sendMetadataFunc: func(any) error {
							return nil
						},
					}, &mockForwarder{
						state: defaultforwarder.Started,
					}
			},
			expectedError:      "",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusInternalServerError,
			serverResponse:     `Internal Server Error`,
		},
		{
			name: "Failed to send payload",
			setupTest: func() (*zap.Logger, *observer.ObservedLogs, serializer.MetricSerializer, defaultForwarderInterface) {
				core, logs := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return logger, logs, &mockSerializer{
						sendMetadataFunc: func(pl any) error {
							if _, ok := pl.(*payload.OtelCollectorPayload); ok {
								return errors.New("failed to send payload")
							}
							return nil
						},
					}, &mockForwarder{
						state: defaultforwarder.Started,
					}
			},
			expectedError:      "failed to send otel_collector payload: failed to send payload",
			expectedLogs:       []string{},
			serverResponseCode: http.StatusInternalServerError,
			serverResponse:     `Internal Server Error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs, serializer, forwarder := tt.setupTest()
			componentStatus := map[string]any{
				"status": "ok",
			}
			c := confmap.New()
			s := NewServer(
				logger,
				&Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: DefaultServerEndpoint,
					},
					Path: "/metadata",
				},
				serializer,
				forwarder,
				"test-hostname-source",
				"test-hostname",
				"test-uuid",
				&componentStatus,
				&service.ModuleInfos{},
				c,
				&payload.OtelCollector{},
			)
			ocPayload, err := s.PrepareAndSendFleetAutomationPayloads()
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
  "host_key": "",
  "hostname": "",
  "hostname_source": "",
  "collector_id": "",
  "collector_version": "",
  "config_site": "",
  "api_key_uuid": "",
  "full_components": [],
  "active_components": null,
  "build_info": {
    "command": "",
    "description": "",
    "version": ""
  },
  "full_configuration": "",
  "health_status": "{}"
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
		setupTest      func() (*zap.Logger, serializer.MetricSerializer, defaultForwarderInterface)
		hostnameSource string
		expectedCode   int
		expectedBody   string
	}{
		{
			name: "Successful metadata handling",
			setupTest: func() (*zap.Logger, serializer.MetricSerializer, defaultForwarderInterface) {
				core, _ := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return logger, &mockSerializer{
						sendMetadataFunc: func(any) error {
							return nil
						},
					}, &mockForwarder{
						state: defaultforwarder.Started,
					}
			},
			hostnameSource: "config",
			expectedCode:   http.StatusOK,
			expectedBody:   successfulInstanceResponse,
		},
		{
			name: "Failed metadata handling - serializer error",
			setupTest: func() (*zap.Logger, serializer.MetricSerializer, defaultForwarderInterface) {
				core, _ := observer.New(zapcore.InfoLevel)
				logger := zap.New(core)
				return logger, &mockSerializer{
						sendMetadataFunc: func(any) error {
							return fmt.Errorf("failed to send metadata")
						},
					}, &mockForwarder{
						state: defaultforwarder.Started,
					}
			},
			hostnameSource: "config",
			expectedCode:   http.StatusInternalServerError,
			expectedBody:   "Failed to prepare and send fleet automation payload\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, serializer, forwarder := tt.setupTest()
			c := confmap.New()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/metadata", nil)
			r.Header.Set("Content-Type", "application/json")
			srv := &Server{
				logger:               logger,
				serializer:           serializer,
				forwarder:            forwarder,
				hostnameSource:       tt.hostnameSource,
				hostname:             "test-hostname",
				uuid:                 "test-uuid",
				componentStatus:      &map[string]any{},
				moduleInfo:           &service.ModuleInfos{},
				collectorConfig:      c,
				otelCollectorPayload: &payload.OtelCollector{},
			}
			srv.HandleMetadata(w, r)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}
