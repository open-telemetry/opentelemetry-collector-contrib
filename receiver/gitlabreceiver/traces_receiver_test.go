// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

const (
	validPipelineWebhookEvent                    = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"},"builds":[{"id":1,"stage":"build","name":"build-job","status":"success","created_at":"2022-01-01 12:00:00 UTC","started_at":"2022-01-01 12:01:00 UTC","finished_at":"2022-01-01 12:10:00 UTC"},{"id":2,"stage":"test","name":"test-job","status":"success","created_at":"2022-01-01 12:11:00 UTC","started_at":"2022-01-01 12:12:00 UTC","finished_at":"2022-01-01 12:20:00 UTC"}],"commit":{"title":"Test commit"}}`
	validPipelineWebhookEventWithoutJobs         = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`
	invalidPipelineWebhookEventMissingFinishedAt = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`
)

// Helper function to create a gitlabTracesReceiver
func setupGitlabTracesReceiver(t *testing.T) *gitlabTracesReceiver {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	var pipelineEvent gitlab.PipelineEvent
	err = json.Unmarshal([]byte(validPipelineWebhookEvent), &pipelineEvent)
	require.NoError(t, err)

	return receiver
}

func TestHandleWebhook(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.Endpoint = "localhost:0"

	tests := []struct {
		name         string
		method       string
		headers      map[string]string
		body         string
		expectedCode int
		spanCount    int
	}{
		{
			name:   "empty_body",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         "",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "invalid_json",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         "{invalid-json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "unexpected_event_type",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Issue Hook",
			},
			body:         "{}",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:   "pipeline_without_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "running_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"running","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "pending_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"pending","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "created_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"created","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "waiting_for_resource_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"waiting_for_resource","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "preparing_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"preparing","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "scheduled_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"scheduled","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC"}}`,
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "successful_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         validPipelineWebhookEvent,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "failed_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"failed","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "canceled_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"canceled","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "skipped_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"skipped","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "unknown_status_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"unknown_status","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline"},"project":{"id":123,"path_with_namespace":"test/project"}}`,
			expectedCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			logger := zap.NewNop()

			mockObsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
				ReceiverID:             component.NewID(metadata.Type),
				ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
			})
			require.NoError(t, err)

			receiver := &gitlabTracesReceiver{
				cfg:           cfg,
				logger:        logger,
				obsrecv:       mockObsrecv,
				traceConsumer: new(consumertest.TracesSink),
				gitlabClient:  &gitlab.Client{},
			}

			req := httptest.NewRequest(tt.method, "http://localhost/webhook", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			receiver.handleWebhook(w, req)

			resp := w.Result()
			require.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func TestHealthCheck(t *testing.T) {
	r := setupGitlabTracesReceiver(t)

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
		name            string
		method          string
		headers         map[string]string
		secret          string
		requiredHeaders map[string]configopaque.String
		expectedEvent   gitlab.EventType
		wantErr         string
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
				defaultGitLabSecretTokenHeader: "secret123",
			},
			secret:  "secret123",
			wantErr: "missing header: X-Gitlab-Event",
		},
		{
			name:   "invalid_secret",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabSecretTokenHeader: "wrongsecret",
				defaultGitLabEventHeader:       "Pipeline Hook",
			},
			secret:  "secret123",
			wantErr: "invalid header: X-Gitlab-Token",
		},
		{
			name:   "valid_request",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabSecretTokenHeader: "secret123",
				defaultGitLabEventHeader:       "Pipeline Hook",
			},
			secret:        "secret123",
			expectedEvent: "Pipeline Hook",
		},
		{
			name:   "valid_request_no_secret_configured",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			secret:        "",
			expectedEvent: "Pipeline Hook",
		},
		{
			name:   "valid_request_with_required_headers",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
				"Custom-Header":          "custom-value",
			},
			requiredHeaders: map[string]configopaque.String{
				"Custom-Header": "custom-value",
			},
			expectedEvent: "Pipeline Hook",
		},
		{
			name:   "invalid_request_missing_required_header",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			requiredHeaders: map[string]configopaque.String{
				"Custom-Header": "custom-value",
			},
			wantErr: "invalid header: Custom-Header",
		},
		{
			name:   "invalid_request_wrong_required_header_value",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
				"Custom-Header":          "wrong-value",
			},
			requiredHeaders: map[string]configopaque.String{
				"Custom-Header": "custom-value",
			},
			wantErr: "invalid header: Custom-Header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.WebHook.Secret = tt.secret
			if tt.requiredHeaders != nil {
				cfg.WebHook.RequiredHeaders = tt.requiredHeaders
			}

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
	receiver := setupGitlabTracesReceiver(t)

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
