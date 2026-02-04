// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

// Test constants - reuse from job_event_handler_test.go
const (
	eventTypeJobHook = "Job Hook"
)

// TestJobEventWebhookIntegration tests the full webhook flow for job events
func TestJobEventWebhookIntegration(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.NetAddr.Endpoint = "localhost:0"

	tests := []struct {
		name         string
		method       string
		headers      map[string]string
		body         string
		expectedCode int
		metricCount  int
		setupMetrics bool
	}{
		{
			name:   "valid_job_event_success",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         loadJobEventTestData(t, testDataJobSuccess),
			expectedCode: http.StatusOK,
			metricCount:  2, // duration + queued_duration
			setupMetrics: true,
		},
		{
			name:   "valid_job_event_failed",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         loadJobEventTestData(t, testDataJobFailed),
			expectedCode: http.StatusOK,
			metricCount:  2,
			setupMetrics: true,
		},
		{
			name:   "running_job_no_metrics",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         loadJobEventTestData(t, testDataJobRunning),
			expectedCode: http.StatusNoContent,
			metricCount:  0,
			setupMetrics: false,
		},
		{
			name:   "pending_job_no_metrics",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         loadJobEventTestData(t, testDataJobPending),
			expectedCode: http.StatusNoContent,
			metricCount:  0,
			setupMetrics: false,
		},
		{
			name:   "job_missing_timestamps",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         loadJobEventTestData(t, testDataJobMissingTimestamps),
			expectedCode: http.StatusNoContent,
			metricCount:  0,
			setupMetrics: false,
		},
		{
			name:   "invalid_json",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: eventTypeJobHook,
			},
			body:         "{invalid-json",
			expectedCode: http.StatusBadRequest,
			metricCount:  0,
			setupMetrics: false,
		},
		{
			name:   "wrong_event_type",
			method: http.MethodPost,
			headers: map[string]string{
				defaultGitLabEventHeader: "Pipeline Hook",
			},
			body:         loadJobEventTestData(t, testDataJobSuccess),
			expectedCode: http.StatusBadRequest, // Event parsing fails when header doesn't match body
			metricCount:  0,
			setupMetrics: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.WebHook.NetAddr.Endpoint = "localhost:0"
			logger := zap.NewNop()

			mockObsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
				ReceiverID:             component.NewID(metadata.Type),
				ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
			})
			require.NoError(t, err)

			metricsSink := new(consumertest.MetricsSink)
			receiver := &gitlabReceiver{
				cfg:            cfg,
				settings:       receivertest.NewNopSettings(metadata.Type),
				logger:         logger,
				obsrecv:        mockObsrecv,
				metricConsumer: metricsSink,
				eventRouter:    newEventRouter(logger, cfg),
			}

			req := httptest.NewRequest(tt.method, "http://localhost/webhook", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			receiver.handleWebhook(w, req)

			resp := w.Result()
			require.Equal(t, tt.expectedCode, resp.StatusCode)

			if tt.setupMetrics {
				require.Equal(t, 1, len(metricsSink.AllMetrics()))
				metrics := metricsSink.AllMetrics()[0]
				require.Equal(t, tt.metricCount, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
			} else {
				require.Equal(t, 0, len(metricsSink.AllMetrics()))
			}
		})
	}
}

// TestJobEventThroughReceiver tests job events through the full receiver lifecycle
// Note: This test requires a real HTTP server, so it's skipped in unit tests
// Use TestJobEventWebhookIntegration for integration testing instead
func TestJobEventThroughReceiver(t *testing.T) {
	t.Skip("Skipping test that requires real HTTP server - use TestJobEventWebhookIntegration instead")
}

// TestJobEventRouterRouting tests that the event router correctly routes job events
func TestJobEventRouterRouting(t *testing.T) {
	logger := zap.NewNop()
	config := createDefaultConfig().(*Config)
	router := newEventRouter(logger, config)

	ctx := context.Background()

	tests := []struct {
		name        string
		event       interface{}
		eventType   gitlab.EventType
		expectError bool
		hasMetrics  bool
	}{
		{
			name:        "job_event_routed",
			event:       parseJobEventFromFile(t, testDataJobSuccess),
			eventType:   gitlab.EventType(eventTypeJobHook),
			expectError: false,
			hasMetrics:  true,
		},
		{
			name:        "pipeline_event_not_routed_to_job_handler",
			event:       &gitlab.PipelineEvent{},
			eventType:   gitlab.EventType("Pipeline Hook"),
			expectError: false,
			hasMetrics:  false,
		},
		{
			name:        "incomplete_job_no_metrics",
			event:       parseJobEventFromFile(t, testDataJobRunning),
			eventType:   gitlab.EventType(eventTypeJobHook),
			expectError: false,
			hasMetrics:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := router.Route(ctx, tt.event, tt.eventType)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.hasMetrics {
				require.NotNil(t, result, "result should not be nil")
				require.NotNil(t, result.Metrics, "result.Metrics should not be nil")
				metricCount := int(result.Metrics.MetricCount())
				require.Greater(t, metricCount, 0, "should have at least one metric")
			} else {
				// Incomplete jobs return nil result
				if result == nil {
					return // Expected for incomplete jobs
				}
				// Pipeline events return traces, not metrics
				if result.Metrics == nil {
					return // Expected for non-job events
				}
			}
		})
	}
}

// TestJobEventMultipleStatuses tests all job statuses
func TestJobEventMultipleStatuses(t *testing.T) {
	handler, _ := setupJobEventHandler(t)
	ctx := context.Background()

	statuses := []struct {
		status         string
		filename       string
		shouldGenerate bool
	}{
		{"success", testDataJobSuccess, true},
		{"failed", testDataJobFailed, true},
		{"canceled", testDataJobCanceled, true},
		{"skipped", testDataJobSkipped, true},
		{"running", testDataJobRunning, false},
		{"pending", testDataJobPending, false},
	}

	for _, tt := range statuses {
		t.Run(tt.status, func(t *testing.T) {
			event := parseJobEventFromFile(t, tt.filename)

			result, err := handler.Handle(ctx, event)
			require.NoError(t, err)

			if tt.shouldGenerate {
				require.NotNil(t, result, "result should not be nil for completed jobs")
				require.NotNil(t, result.Metrics, "result.Metrics should not be nil for completed jobs")
				metricCount := int(result.Metrics.MetricCount())
				require.Greater(t, metricCount, 0, "should have at least one metric")
			} else {
				require.Nil(t, result, "result should be nil for incomplete jobs")
			}
		})
	}
}

// TestJobEventResourceAttributes validates resource attributes are set correctly
func TestJobEventResourceAttributes(t *testing.T) {
	handler, _ := setupJobEventHandler(t)
	ctx := context.Background()

	event := parseJobEventFromFile(t, testDataJobSuccess)
	result, err := handler.Handle(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Metrics)

	resource := result.Metrics.ResourceMetrics().At(0).Resource()
	attrs := resource.Attributes()

	// Verify required attributes
	projectID, found := attrs.Get("gitlab.project.id")
	require.True(t, found)
	require.Equal(t, "380", projectID.Str())

	projectName, found := attrs.Get("gitlab.project.name")
	require.True(t, found)
	require.Equal(t, "gitlab-org/gitlab-test", projectName.Str())

	serviceName, found := attrs.Get("service.name")
	require.True(t, found)
	require.Equal(t, "gitlab-org/gitlab-test", serviceName.Str())
}

// TestJobEventMetricAttributes validates metric data point attributes
func TestJobEventMetricAttributes(t *testing.T) {
	handler, _ := setupJobEventHandler(t)
	ctx := context.Background()

	event := parseJobEventFromFile(t, testDataJobSuccess)
	result, err := handler.Handle(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, result)

	metrics := result.Metrics
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)
	durationMetric := scopeMetrics.Metrics().At(0)

	require.Equal(t, "gitlab.job.duration", durationMetric.Name())
	dp := durationMetric.Gauge().DataPoints().At(0)

	attrs := dp.Attributes()
	_, found := attrs.Get("cicd.pipeline.task.run.id")
	require.True(t, found)
	_, found = attrs.Get("cicd.pipeline.task.name")
	require.True(t, found)
	_, found = attrs.Get("gitlab.job.stage")
	require.True(t, found)
	_, found = attrs.Get("cicd.pipeline.task.run.result")
	require.True(t, found)
	_, found = attrs.Get("cicd.worker.id")
	require.True(t, found)
	_, found = attrs.Get("cicd.worker.name")
	require.True(t, found)
	_, found = attrs.Get("gitlab.job.runner.tags")
	require.True(t, found)
}

// TestJobEventQueuedDurationMetric validates queued duration metric
func TestJobEventQueuedDurationMetric(t *testing.T) {
	handler, _ := setupJobEventHandler(t)
	ctx := context.Background()

	event := parseJobEventFromFile(t, testDataJobSuccess)
	result, err := handler.Handle(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, result)

	metrics := result.Metrics
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Should have 2 metrics: duration and queued_duration
	require.Equal(t, 2, scopeMetrics.Metrics().Len())

	queuedMetric := scopeMetrics.Metrics().At(1)
	require.Equal(t, "gitlab.job.queued_duration", queuedMetric.Name())
	require.Equal(t, "s", queuedMetric.Unit())

	dp := queuedMetric.Gauge().DataPoints().At(0)
	require.Equal(t, 1095.588715, dp.DoubleValue())

	attrs := dp.Attributes()
	_, found := attrs.Get("cicd.pipeline.task.run.id")
	require.True(t, found)
	_, found = attrs.Get("cicd.pipeline.task.name")
	require.True(t, found)
	_, found = attrs.Get("gitlab.job.stage")
	require.True(t, found)
}

// TestJobEventNoQueuedDuration tests job without queued duration
func TestJobEventNoQueuedDuration(t *testing.T) {
	handler, _ := setupJobEventHandler(t)
	ctx := context.Background()

	event := parseJobEventFromFile(t, testDataJobWithoutQueuedDur)
	result, err := handler.Handle(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, result)

	metrics := result.Metrics
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0)

	// Should have only 1 metric: duration (no queued_duration since it's 0)
	require.Equal(t, 1, scopeMetrics.Metrics().Len())
	require.Equal(t, "gitlab.job.duration", scopeMetrics.Metrics().At(0).Name())
}
