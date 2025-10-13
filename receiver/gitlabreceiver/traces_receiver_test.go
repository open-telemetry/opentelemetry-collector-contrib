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

const (
	validPipelineWebhookEvent                    = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","source":"push","ref":"main","tag":false,"sha":"abc123def456"},"project":{"id":123,"name":"my-project","path_with_namespace":"test/project","namespace":"test","web_url":"https://gitlab.example.com/test/project","visibility":"private","default_branch":"main"},"builds":[{"id":1,"stage":"build","name":"build-job","status":"success","created_at":"2022-01-01 12:00:00 UTC","started_at":"2022-01-01 12:01:00 UTC","finished_at":"2022-01-01 12:10:00 UTC","runner":{"id":100,"description":"shared-runner-1","active":true,"is_shared":true,"runner_type":"instance_type","tags":["docker","linux"]},"queued_duration":60.5,"allow_failure":false},{"id":2,"stage":"test","name":"test-job","status":"success","created_at":"2022-01-01 12:11:00 UTC","started_at":"2022-01-01 12:12:00 UTC","finished_at":"2022-01-01 12:20:00 UTC","runner":{"id":101,"description":"project-runner-1","active":true,"is_shared":false,"runner_type":"project_type","tags":["ruby","postgres"]},"queued_duration":30.2,"allow_failure":false}],"commit":{"id":"abc123def456","message":"Fix critical bug\n\nDetailed description of the fix","timestamp":"2022-01-01T11:50:00Z","author":{"name":"John Doe","email":"john.doe@example.com"}},"user":{"id":42,"username":"jdoe","name":"John Doe"}}`
	validPipelineWebhookEventWithoutJobs         = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","source":"push","ref":"main","tag":false,"sha":"abc123"},"project":{"id":123,"name":"my-project","path_with_namespace":"test/project","namespace":"test","web_url":"https://gitlab.example.com/test/project","visibility":"private","default_branch":"main"},"commit":{"id":"abc123","message":"Test commit","timestamp":"2022-01-01T11:50:00Z","author":{"name":"Test User","email":"test@example.com"}},"user":{"id":1,"username":"testuser","name":"Test User"}}`
	invalidPipelineWebhookEventMissingFinishedAt = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","name":"Test Pipeline","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`
	minimalValidPipelineWebhookEvent             = `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"","source":"","ref":"main","tag":false,"sha":"abc123","url":""},"project":{"id":123,"name":"","path_with_namespace":"test/project","namespace":"","web_url":"","visibility":"","default_branch":""},"commit":{"id":"abc123","message":"","timestamp":null,"author":{"name":"","email":""}},"user":null,"merge_request":{"id":0,"state":"","title":"","target_branch":"","source_branch":""}}`

	// Multi-pipeline webhook event with parent pipeline information
	validMultiPipelineWebhookEvent = `{"object_attributes":{"id":2,"status":"success","created_at":"2022-01-02 14:00:00 UTC","finished_at":"2022-01-02 14:30:00 UTC","name":"Child Pipeline","source":"parent_pipeline","ref":"main","tag":false,"sha":"def789ghi012"},"project":{"id":124,"name":"child-project","path_with_namespace":"test/child-project","namespace":"test","web_url":"https://gitlab.example.com/test/child-project","visibility":"internal","default_branch":"main"},"source_pipeline":{"project":{"id":123,"path_with_namespace":"test/parent-project","web_url":"https://gitlab.example.com/test/parent-project"},"pipeline_id":1,"job_id":99},"builds":[],"commit":{"id":"def789ghi012","message":"Trigger child pipeline","timestamp":"2022-01-02T13:50:00Z","author":{"name":"Jane Smith","email":"jane.smith@example.com"}},"user":{"id":43,"username":"jsmith","name":"Jane Smith"}}`

	// Pipeline with environment deployment
	validPipelineWithEnvironmentEvent = `{"object_attributes":{"id":3,"status":"success","created_at":"2022-01-03 10:00:00 UTC","finished_at":"2022-01-03 10:45:00 UTC","name":"Deploy Pipeline","source":"merge_request_event","ref":"main","tag":false,"sha":"xyz789abc123"},"project":{"id":125,"name":"web-app","path_with_namespace":"prod/web-app","namespace":"prod","web_url":"https://gitlab.example.com/prod/web-app","visibility":"public","default_branch":"main"},"builds":[{"id":10,"stage":"deploy","name":"deploy-staging","status":"success","created_at":"2022-01-03 10:20:00 UTC","started_at":"2022-01-03 10:21:00 UTC","finished_at":"2022-01-03 10:30:00 UTC","runner":{"id":200,"description":"deploy-runner","active":true,"is_shared":false,"runner_type":"group_type","tags":["kubernetes","production"]},"queued_duration":45.0,"allow_failure":false,"environment":{"name":"staging","action":"start","deployment_tier":"staging"}},{"id":11,"stage":"deploy","name":"deploy-production","status":"failed","created_at":"2022-01-03 10:31:00 UTC","started_at":"2022-01-03 10:32:00 UTC","finished_at":"2022-01-03 10:40:00 UTC","runner":{"id":201,"description":"prod-runner","active":true,"is_shared":false,"runner_type":"group_type","tags":["kubernetes","production","critical"]},"queued_duration":60.0,"allow_failure":true,"failure_reason":"script_failure","environment":{"name":"production","action":"start","deployment_tier":"production"}}],"commit":{"id":"xyz789abc123","message":"Deploy v2.0.0 to production","timestamp":"2022-01-03T09:45:00Z","author":{"name":"Deploy Bot","email":"deploy@example.com"}},"user":{"id":1,"username":"deploy-bot","name":"Deploy Bot"},"merge_request":{"iid":42,"target_branch":"main","source_branch":"feature/v2"}}`
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
			body:         `{"object_attributes":{"id":1,"status":"failed","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "canceled_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"canceled","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "skipped_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"skipped","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedCode: http.StatusOK,
			spanCount:    1,
		},
		{
			name:   "unknown_status_pipeline",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         `{"object_attributes":{"id":1,"status":"unknown_status","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","name":"Test Pipeline","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
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

			receiver := &gitlabTracesReceiver{
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
	receiver := setupGitlabTracesReceiver(t)

	tests := []struct {
		name            string
		jsonEvent       string
		expectedErrText string
	}{
		{
			name:      "valid_event",
			jsonEvent: validPipelineWebhookEvent,
		},
		{
			name:            "missing_pipeline_id",
			jsonEvent:       `{"object_attributes":{"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: pipeline ID",
		},
		{
			name:            "missing_created_at",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: pipeline created_at",
		},
		{
			name:            "missing_ref",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: pipeline ref",
		},
		{
			name:            "missing_sha",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: pipeline SHA",
		},
		{
			name:            "missing_project_id",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"path_with_namespace":"test/project"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: project ID",
		},
		{
			name:            "missing_project_path",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"id":123},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: project path_with_namespace",
		},
		{
			name:            "missing_commit_id",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"},"commit":{}}`,
			expectedErrText: "missing required field: commit ID",
		},
		{
			name:            "nil_commit",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"project":{"id":123,"path_with_namespace":"test/project"}}`,
			expectedErrText: "missing required field: commit ID",
		},
		{
			name:            "nil_project",
			jsonEvent:       `{"object_attributes":{"id":1,"status":"success","created_at":"2022-01-01 12:00:00 UTC","finished_at":"2022-01-01 13:00:00 UTC","ref":"main","sha":"abc123"},"commit":{"id":"abc123"}}`,
			expectedErrText: "missing required field: project ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pipelineEvent gitlab.PipelineEvent
			err := json.Unmarshal([]byte(tt.jsonEvent), &pipelineEvent)
			require.NoError(t, err, "failed to unmarshal test event")

			err = receiver.validatePipelineEvent(&pipelineEvent)
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
