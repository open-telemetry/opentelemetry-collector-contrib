// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default",
			cfg: &Config{
				Endpoint: DefaultEndpoint,
				Interval: DefaultInterval,
			},
		},
		{
			name: "anything",
			cfg: &Config{
				Endpoint: "apm.collector.na-02.cloud.solarwinds.com:443",
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := createAnExtension(t, tt.cfg)
			require.NoError(t, ex.Shutdown(context.TODO()))
		})
	}
}

// newNopSettings returns a new nop settings for extension.Factory Create* functions.
func newNopSettings() extension.Settings {
	return extension.Settings{
		ID:                component.NewIDWithName(component.MustNewType("nop"), uuid.NewString()),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}

// create extension
func createAnExtension(t *testing.T, c *Config) extension.Extension {
	ex, err := newSolarwindsApmSettingsExtension(c, newNopSettings())
	require.NoError(t, err)
	require.NoError(t, ex.Start(context.TODO(), nil))
	return ex
}

// MockRoundTripper implements http.RoundTripper for testing.
type MockRoundTripper struct {
	Response *http.Response
	Err      error
}

// RoundTrip implements the http.RoundTripper interface.
func (m *MockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

func TestRefresh(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		response            *http.Response
		networkError        error
		filename            string
		expectedLogMessages []string
		fileExist           bool
	}{
		{
			name: "ok",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("{\"value\":1000000,\"flags\":\"SAMPLE_START,SAMPLE_THROUGH_ALWAYS,SAMPLE_BUCKET_ENABLED,TRIGGER_TRACE\",\"timestamp\":1754601362,\"ttl\":120,\"arguments\":{\"BucketCapacity\":2,\"BucketRate\":1,\"TriggerRelaxedBucketCapacity\":20,\"TriggerRelaxedBucketRate\":1,\"TriggerStrictBucketCapacity\":6,\"TriggerStrictBucketRate\":0.1,\"SignatureKey\":\"signatureKey\"}}")),
			},
			networkError: nil,
			filename:     "testdata/refresh_ok.json",
			expectedLogMessages: []string{
				"time to refresh",
				"testdata/refresh_ok.json is refreshed",
				"[{\"value\":1000000,\"flags\":\"SAMPLE_START,SAMPLE_THROUGH_ALWAYS,SAMPLE_BUCKET_ENABLED,TRIGGER_TRACE\",\"timestamp\":1754601362,\"ttl\":120,\"arguments\":{\"BucketCapacity\":2,\"BucketRate\":1,\"TriggerRelaxedBucketCapacity\":20,\"TriggerRelaxedBucketRate\":1,\"TriggerStrictBucketCapacity\":6,\"TriggerStrictBucketRate\":0.1}}]",
			},
			fileExist: true,
		},
		{
			name: "soft disabled",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("{\"value\":0,\"flags\":\"OVERRIDE\",\"timestamp\":1754601483,\"ttl\":120,\"arguments\":{\"BucketCapacity\":0,\"BucketRate\":0,\"TriggerRelaxedBucketCapacity\":0,\"TriggerRelaxedBucketRate\":0,\"TriggerStrictBucketCapacity\":0,\"TriggerStrictBucketRate\":0},\"warning\":\"There is an problem getting the API token authorized. Metrics and tracing for this agent are currently disabled. If you'd like to learn more about resolving this issue, please contact support (see https://support.solarwinds.com/working-with-support).\"}")),
			},
			networkError: nil,
			filename:     "testdata/refresh_warning.json",
			expectedLogMessages: []string{
				"time to refresh",
				"testdata/refresh_warning.json is refreshed (soft disabled)",
				"[{\"value\":0,\"flags\":\"OVERRIDE\",\"timestamp\":1754601483,\"ttl\":120,\"arguments\":{\"BucketCapacity\":0,\"BucketRate\":0,\"TriggerRelaxedBucketCapacity\":0,\"TriggerRelaxedBucketRate\":0,\"TriggerStrictBucketCapacity\":0,\"TriggerStrictBucketRate\":0}}]",
			},
			fileExist: true,
		},
		{
			name: "not a json",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("not a json")),
			},
			networkError: nil,
			expectedLogMessages: []string{
				"time to refresh",
				"unable to unmarshal setting",
			},
			fileExist: false,
		},
		{
			name: "404 Not Found",
			response: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("404 page not found\n")),
			},
			networkError: nil,
			expectedLogMessages: []string{
				"time to refresh",
				"received non-OK HTTP status",
			},
			fileExist: false,
		},
		{
			name:         "net.ErrClosed",
			response:     nil,
			networkError: net.ErrClosed,
			expectedLogMessages: []string{
				"time to refresh",
				"unable to send request",
			},
			fileExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			settings := newNopSettings()
			settings.Logger = zap.New(observedZapCore)
			mockRequest, err := http.NewRequest(http.MethodGet, "http://mock", http.NoBody)
			require.NoError(t, err)
			settingsExtension := &solarwindsapmSettingsExtension{
				config: &Config{
					Endpoint: DefaultEndpoint,
				},
				telemetrySettings: settings.TelemetrySettings,
				client: &http.Client{
					Transport: &MockRoundTripper{
						Response: tt.response,
						Err:      tt.networkError,
					},
				},
				request: mockRequest,
			}
			refresh(settingsExtension, tt.filename)
			require.Equal(t, len(tt.expectedLogMessages), observedLogs.Len())
			for index, observedLog := range observedLogs.All() {
				require.Equal(t, tt.expectedLogMessages[index], observedLog.Message)
			}
			_, err = os.Stat(tt.filename)
			if tt.fileExist {
				require.NoError(t, err)
				require.NoError(t, os.Remove(tt.filename))
			} else {
				require.Error(t, err)
			}
		})
	}
}
