// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
)

func TestHealthCheck(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	r := receiver
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()), "failed to shutdown revceiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest(http.MethodGet, "http://localhost/health", nil))

	response := w.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func TestValidateReq(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		headers       map[string]string
		secret        string
		expectedEvent gitlab.EventType
		wantErr       string
	}{
		{
			name:    "invalid_method",
			method:  http.MethodGet,
			wantErr: "invalid HTTP method",
		},
		{
			name:   "missing_event_header",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitlabTokenHeader: "secret123",
			},
			secret:  "secret123",
			wantErr: "missing X-Gitlab-Event header",
		},
		{
			name:   "invalid_secret",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitlabTokenHeader: "wrongsecret",
				defaultGitlabEventHeader: "Pipeline Hook",
			},
			secret:  "secret123",
			wantErr: "invalid X-Gitlab-Token header",
		},
		{
			name:   "valid_request",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitlabTokenHeader: "secret123",
				defaultGitlabEventHeader: "Pipeline Hook",
			},
			secret:        "secret123",
			expectedEvent: "Pipeline Hook",
		},
		{
			name:   "valid_request_no_secret_configured",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitlabEventHeader: "Pipeline Hook",
			},
			secret:        "",
			expectedEvent: "Pipeline Hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.WebHook.Secret = tt.secret

			receiver := &gitlabTracesReceiver{
				cfg: cfg,
			}

			req := httptest.NewRequest(tt.method, "http://localhost", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			eventType, err := receiver.validateReq(req)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedEvent, eventType)
		})
	}
}

func TestFailBadReq(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedCode   int
		expectedBody   string
		expectedHeader string
		spanCount      int
	}{
		{
			name:           "simple_error",
			err:            errors.New("test error"),
			expectedCode:   http.StatusBadRequest,
			expectedBody:   `"test error"`,
			expectedHeader: "application/json",
			spanCount:      5,
		},
		{
			name:         "nil_error",
			err:          nil,
			expectedCode: http.StatusBadRequest,
			spanCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			logger := zap.NewNop()

			mockObsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
				ReceiverID:             component.NewID(metadata.Type),
				Transport:              "http",
				ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
			})
			require.NoError(t, err)

			receiver := &gitlabTracesReceiver{
				cfg:     cfg,
				logger:  logger,
				obsrecv: mockObsrecv,
			}

			w := httptest.NewRecorder()
			ctx := context.Background()
			receiver.failBadReq(ctx, w, tt.expectedCode, tt.err, tt.spanCount)

			resp := w.Result()
			require.Equal(t, tt.expectedCode, resp.StatusCode)

			if tt.expectedHeader != "" {
				require.Equal(t, tt.expectedHeader, resp.Header.Get("Content-Type"))
			}

			if tt.expectedBody != "" {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, tt.expectedBody, string(body))
			}
		})
	}
}
