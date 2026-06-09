// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

func TestCreateNewTracesReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc     string
		config   Config
		consumer consumer.Traces
		err      error
	}{
		{
			desc:     "Default config succeeds",
			config:   *defaultConfig,
			consumer: consumertest.NewNop(),
			err:      nil,
		},
		{
			desc: "User defined config success",
			config: Config{
				WebHook: WebHook{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Transport: confignet.TransportTypeTCP,
							Endpoint:  "localhost:0",
						},
					},
					Path:       "/events",
					HealthPath: "/health_check",
				},
			},
			consumer: consumertest.NewNop(),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rec, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), &test.config, test.consumer)
			if test.err == nil {
				require.NotNil(t, rec)
			} else {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, rec)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.NetAddr.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	r := receiver
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(t.Context()), "failed to shutdown receiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest(http.MethodGet, "http://localhost/health", http.NoBody))

	response := w.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func TestHandleReq_RequiredHeaders(t *testing.T) {
	tests := []struct {
		name            string
		requiredHeaders map[string]configopaque.String
		reqHeaders      map[string]string
		wantStatus      int
	}{
		{
			name:            "valid_request_with_required_headers",
			requiredHeaders: map[string]configopaque.String{"X-Proxy-Auth": "abc"},
			reqHeaders: map[string]string{
				"X-Proxy-Auth":   "abc",
				"X-GitHub-Event": "ping",
				"Content-Type":   "application/json",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:            "missing_required_header",
			requiredHeaders: map[string]configopaque.String{"X-Proxy-Auth": "abc"},
			reqHeaders: map[string]string{
				"X-GitHub-Event": "ping",
				"Content-Type":   "application/json",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:            "wrong_required_header_value",
			requiredHeaders: map[string]configopaque.String{"X-Proxy-Auth": "abc"},
			reqHeaders: map[string]string{
				"X-Proxy-Auth":   "wrong",
				"X-GitHub-Event": "ping",
				"Content-Type":   "application/json",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:            "no_required_headers_configured",
			requiredHeaders: nil,
			reqHeaders: map[string]string{
				"X-GitHub-Event": "ping",
				"Content-Type":   "application/json",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.WebHook.NetAddr.Endpoint = "localhost:0"
			if tt.requiredHeaders != nil {
				cfg.WebHook.RequiredHeaders = tt.requiredHeaders
			}

			sink := new(consumertest.TracesSink)
			r, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), cfg, sink)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "http://localhost/events",
				strings.NewReader(`{}`))
			for k, v := range tt.reqHeaders {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			r.handleReq(w, req)

			require.Equal(t, tt.wantStatus, w.Result().StatusCode)
			if tt.wantStatus == http.StatusBadRequest {
				require.Equal(t, 0, sink.SpanCount(), "consumer must not be called when header check fails")
			}
		})
	}
}
