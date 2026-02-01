// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
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

// Test data file names for pipeline events (additional constants for traces_receiver_test.go)
const (
	testDataPipelineWithoutFinishedAt     = "without_finished_at.json"
	testDataPipelineRunningWithFinishedAt = "running_with_finished_at.json"
	testDataPipelinePendingWithFinishedAt = "pending_with_finished_at.json"
	testDataPipelineCreatedWithFinishedAt = "created_with_finished_at.json"
	testDataPipelineWaitingForResource    = "waiting_for_resource_with_finished_at.json"
	testDataPipelinePreparing             = "preparing_with_finished_at.json"
	testDataPipelineScheduled             = "scheduled_with_finished_at.json"
	testDataPipelineFailed                = "failed.json"
	testDataPipelineCanceled              = "canceled.json"
	testDataPipelineSkipped               = "skipped.json"
	testDataPipelineUnknownStatus         = "unknown_status.json"
	testDataPipelineMissingID             = "missing_pipeline_id.json"
	testDataPipelineMissingCreatedAt      = "missing_created_at.json"
	testDataPipelineMissingRef            = "missing_ref.json"
	testDataPipelineMissingSHA            = "missing_sha.json"
	testDataPipelineMissingProjectID      = "missing_project_id.json"
	testDataPipelineMissingProjectPath    = "missing_project_path.json"
	testDataPipelineMissingCommitID       = "missing_commit_id.json"
	testDataPipelineNilCommit             = "nil_commit.json"
	testDataPipelineNilProject            = "nil_project.json"
	testDataPipelineEmpty                 = "empty.json"
)

// Helper function to create a gitlabReceiver
func setupGitlabTracesReceiver(t *testing.T) *gitlabReceiver {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.NetAddr.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newTracesReceiver(receivertest.NewNopSettings(metadata.Type), defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	return receiver
}

func TestHandleWebhook(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.NetAddr.Endpoint = "localhost:0"

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
			body:         loadPipelineEventTestData(t, testDataPipelineEmpty),
			expectedCode: http.StatusNoContent, // Unknown event types are ignored (no handler found)
		},
		{
			name:   "pipeline_without_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineWithoutFinishedAt),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "running_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineRunningWithFinishedAt),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "pending_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelinePendingWithFinishedAt),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "created_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineCreatedWithFinishedAt),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "waiting_for_resource_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineWaitingForResource),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "preparing_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelinePreparing),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "scheduled_pipeline_with_finishedat",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineScheduled),
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "successful_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineValidWithJobs),
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "failed_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineFailed),
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "canceled_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineCanceled),
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "skipped_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineSkipped),
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "unknown_status_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadPipelineEventTestData(t, testDataPipelineUnknownStatus),
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

			receiver := &gitlabReceiver{
				cfg:           cfg,
				logger:        logger,
				obsrecv:       mockObsrecv,
				traceConsumer: new(consumertest.TracesSink),
				gitlabClient:  &gitlab.Client{},
				eventRouter:   newEventRouter(logger, cfg),
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

	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(t.Context()), "failed to shutdown revceiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest(http.MethodGet, "http://localhost/health", http.NoBody))

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

			// Create gitlabReceiver instance to use validateReq method
			receiver := &gitlabReceiver{
				cfg: cfg,
			}

			req := httptest.NewRequest(tt.method, "http://localhost", http.NoBody)
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

func TestValidatePipelineEvent(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		expectedErrText string
	}{
		{
			name:     "valid_event",
			filename: testDataPipelineValidWithJobs,
		},
		{
			name:            "missing_pipeline_id",
			filename:        testDataPipelineMissingID,
			expectedErrText: "missing required field: pipeline ID",
		},
		{
			name:            "missing_created_at",
			filename:        testDataPipelineMissingCreatedAt,
			expectedErrText: "missing required field: pipeline created_at",
		},
		{
			name:            "missing_ref",
			filename:        testDataPipelineMissingRef,
			expectedErrText: "missing required field: pipeline ref",
		},
		{
			name:            "missing_sha",
			filename:        testDataPipelineMissingSHA,
			expectedErrText: "missing required field: pipeline SHA",
		},
		{
			name:            "missing_project_id",
			filename:        testDataPipelineMissingProjectID,
			expectedErrText: "missing required field: project ID",
		},
		{
			name:            "missing_project_path",
			filename:        testDataPipelineMissingProjectPath,
			expectedErrText: "missing required field: project path_with_namespace",
		},
		{
			name:            "missing_commit_id",
			filename:        testDataPipelineMissingCommitID,
			expectedErrText: "missing required field: commit ID",
		},
		{
			name:            "nil_commit",
			filename:        testDataPipelineNilCommit,
			expectedErrText: "missing required field: commit ID",
		},
		{
			name:            "nil_project",
			filename:        testDataPipelineNilProject,
			expectedErrText: "missing required field: project ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := loadPipelineEventTestData(t, tt.filename)

			var pipelineEvent gitlab.PipelineEvent
			err := json.Unmarshal([]byte(jsonData), &pipelineEvent)
			require.NoError(t, err, "failed to unmarshal test event")

			err = validatePipelineEvent(&pipelineEvent)
			if tt.expectedErrText != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrText)
			} else {
				require.NoError(t, err)
			}
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
			ctx := t.Context()
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
