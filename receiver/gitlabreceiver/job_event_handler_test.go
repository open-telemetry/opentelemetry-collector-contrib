// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

// Test data file names
const (
	testDataJobSuccess           = "success.json"
	testDataJobFailed            = "failed.json"
	testDataJobCanceled          = "canceled.json"
	testDataJobSkipped           = "skipped.json"
	testDataJobRunning           = "running.json"
	testDataJobPending           = "pending.json"
	testDataJobMissingTimestamps = "missing_timestamps.json"
	testDataJobWithoutRunner     = "without_runner.json"
	testDataJobWithoutQueuedDur  = "without_queued_duration.json"
)

// Metric and attribute names used in tests
const (
	metricJobDuration  = "gitlab.job.duration"
	metricJobQueuedDur = "gitlab.job.queued_duration"
	// Semantic convention attributes
	attrJobID      = "cicd.pipeline.task.run.id"
	attrJobName    = "cicd.pipeline.task.name"
	attrJobStatus  = "cicd.pipeline.task.run.result"
	attrWorkerID   = "cicd.worker.id"
	attrWorkerName = "cicd.worker.name"
	// Custom attributes (not yet in semantic conventions)
	attrJobStage    = "gitlab.job.stage"
	attrRunnerTags  = "gitlab.job.runner.tags"
	testProjectName = "gitlab-org/gitlab-test"
)

// loadJobEventTestData loads a job event test data file from testdata
func loadJobEventTestData(t *testing.T, filename string) string {
	data, err := os.ReadFile(filepath.Join("testdata", "events", "job", filename))
	require.NoError(t, err, "failed to load test data: %s", filename)
	return string(data)
}

func setupJobEventHandler(t *testing.T) *jobEventHandler {
	logger := zaptest.NewLogger(t)
	config := createDefaultConfig().(*Config)
	handler := newJobEventHandler(logger, config)
	return handler
}

// parseJobEvent parses a job event from JSON string
func parseJobEvent(t *testing.T, jsonStr string) *gitlab.JobEvent {
	var event gitlab.JobEvent
	err := json.Unmarshal([]byte(jsonStr), &event)
	require.NoError(t, err, "failed to parse job event JSON")
	return &event
}

// parseJobEventFromFile loads and parses a job event from a testdata file
func parseJobEventFromFile(t *testing.T, filename string) *gitlab.JobEvent {
	jsonStr := loadJobEventTestData(t, filename)
	return parseJobEvent(t, jsonStr)
}

func TestJobEventHandlerCanHandle(t *testing.T) {
	handler := setupJobEventHandler(t)

	tests := []struct {
		name     string
		event    any
		expected bool
	}{
		{
			name:     "job_event",
			event:    &gitlab.JobEvent{},
			expected: true,
		},
		{
			name:     "pipeline_event",
			event:    &gitlab.PipelineEvent{},
			expected: false,
		},
		{
			name:     "nil_event",
			event:    nil,
			expected: false,
		},
		{
			name:     "string_event",
			event:    "not an event",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandle(tt.event)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestJobEventHandlerHandle(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()

	tests := []struct {
		name            string
		jsonEvent       string
		expectError     bool
		expectedErrText string
		expectedMetrics int
		validateMetrics func(*testing.T, *pmetric.Metrics)
	}{
		{
			name:            "successful_job_with_queued_duration",
			jsonEvent:       loadJobEventTestData(t, testDataJobSuccess),
			expectError:     false,
			expectedMetrics: 2, // duration + queued_duration
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				require.Equal(t, 1, m.ResourceMetrics().Len())
				resource := m.ResourceMetrics().At(0).Resource()
				attrs := resource.Attributes()
				_, found := attrs.Get("gitlab.project.id")
				require.True(t, found)
				_, found = attrs.Get("gitlab.project.name")
				require.True(t, found)
				_, found = attrs.Get("service.name")
				require.True(t, found)

				scopeMetrics := m.ResourceMetrics().At(0).ScopeMetrics()
				require.Equal(t, 1, scopeMetrics.Len())
				metrics := scopeMetrics.At(0).Metrics()
				require.Equal(t, 2, metrics.Len())

				// Check duration metric
				durationMetric := metrics.At(0)
				require.Equal(t, metricJobDuration, durationMetric.Name())
				require.Equal(t, "s", durationMetric.Unit())
				gauge := durationMetric.Gauge()
				require.Equal(t, 1, gauge.DataPoints().Len())
				dp := gauge.DataPoints().At(0)
				require.Equal(t, 210.0, dp.DoubleValue()) // 3.5 minutes = 210 seconds

				// Check attributes
				dpAttrs := dp.Attributes()
				jobID, found := dpAttrs.Get(attrJobID)
				require.True(t, found)
				require.Equal(t, int64(1977), jobID.Int())
				jobName, found := dpAttrs.Get(attrJobName)
				require.True(t, found)
				require.Equal(t, "test", jobName.Str())
				jobStage, found := dpAttrs.Get(attrJobStage)
				require.True(t, found)
				require.Equal(t, "test", jobStage.Str())
				jobStatus, found := dpAttrs.Get(attrJobStatus)
				require.True(t, found)
				require.Equal(t, "success", jobStatus.Str())
				runnerID, found := dpAttrs.Get(attrWorkerID)
				require.True(t, found)
				require.Equal(t, int64(380987), runnerID.Int())
				runnerDesc, found := dpAttrs.Get(attrWorkerName)
				require.True(t, found)
				require.Equal(t, "shared-runners-manager-6.gitlab.com", runnerDesc.Str())
				runnerTags, found := dpAttrs.Get(attrRunnerTags)
				require.True(t, found)
				require.Equal(t, "linux,docker,shared-runner", runnerTags.Str())

				// Check queued duration metric
				queuedMetric := metrics.At(1)
				require.Equal(t, "gitlab.job.queued_duration", queuedMetric.Name())
				require.Equal(t, "s", queuedMetric.Unit())
				queuedGauge := queuedMetric.Gauge()
				require.Equal(t, 1, queuedGauge.DataPoints().Len())
				queuedDp := queuedGauge.DataPoints().At(0)
				require.Equal(t, 1095.588715, queuedDp.DoubleValue())
			},
		},
		{
			name:            "failed_job",
			jsonEvent:       loadJobEventTestData(t, testDataJobFailed),
			expectError:     false,
			expectedMetrics: 2, // duration + queued_duration
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				durationMetric := metrics.At(0)
				dp := durationMetric.Gauge().DataPoints().At(0)
				require.Equal(t, 240.0, dp.DoubleValue())
				status, found := dp.Attributes().Get(attrJobStatus)
				require.True(t, found)
				require.Equal(t, "failed", status.Str())
			},
		},
		{
			name:            "canceled_job",
			jsonEvent:       loadJobEventTestData(t, testDataJobCanceled),
			expectError:     false,
			expectedMetrics: 1, // Only duration (queued_duration is 0)
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				require.Equal(t, 1, metrics.Len()) // No queued duration metric
				durationMetric := metrics.At(0)
				dp := durationMetric.Gauge().DataPoints().At(0)
				status, found := dp.Attributes().Get(attrJobStatus)
				require.True(t, found)
				require.Equal(t, "canceled", status.Str())
			},
		},
		{
			name:            "skipped_job",
			jsonEvent:       loadJobEventTestData(t, testDataJobSkipped),
			expectError:     false,
			expectedMetrics: 2,
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				durationMetric := metrics.At(0)
				dp := durationMetric.Gauge().DataPoints().At(0)
				status, found := dp.Attributes().Get(attrJobStatus)
				require.True(t, found)
				require.Equal(t, "skipped", status.Str())
			},
		},
		{
			name:            "running_job_skipped",
			jsonEvent:       loadJobEventTestData(t, testDataJobRunning),
			expectError:     false,
			expectedMetrics: 0, // Incomplete jobs are skipped
		},
		{
			name:            "pending_job_skipped",
			jsonEvent:       loadJobEventTestData(t, testDataJobPending),
			expectError:     false,
			expectedMetrics: 0, // Incomplete jobs are skipped
		},
		{
			name:            "job_missing_timestamps_skipped",
			jsonEvent:       loadJobEventTestData(t, testDataJobMissingTimestamps),
			expectError:     false,
			expectedMetrics: 0, // Jobs without timestamps are skipped
		},
		{
			name:            "job_without_runner",
			jsonEvent:       loadJobEventTestData(t, testDataJobWithoutRunner),
			expectError:     false,
			expectedMetrics: 1,
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				durationMetric := metrics.At(0)
				dp := durationMetric.Gauge().DataPoints().At(0)
				// Runner attributes should not be present
				_, found := dp.Attributes().Get(attrWorkerID)
				require.False(t, found)
			},
		},
		{
			name:            "job_without_queued_duration",
			jsonEvent:       loadJobEventTestData(t, testDataJobWithoutQueuedDur),
			expectError:     false,
			expectedMetrics: 1, // Only duration metric
			validateMetrics: func(t *testing.T, m *pmetric.Metrics) {
				metrics := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				require.Equal(t, 1, metrics.Len())
				require.Equal(t, metricJobDuration, metrics.At(0).Name())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := parseJobEvent(t, tt.jsonEvent)

			result, err := handler.Handle(ctx, event)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrText != "" {
					require.Contains(t, err.Error(), tt.expectedErrText)
				}
				return
			}

			require.NoError(t, err)

			if tt.expectedMetrics == 0 {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			require.NotNil(t, result.Metrics)

			metrics := result.Metrics
			require.Equal(t, tt.expectedMetrics, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

			if tt.validateMetrics != nil {
				tt.validateMetrics(t, metrics)
			}
		})
	}
}

func TestJobEventHandlerHandleInvalidEventType(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()

	// Test with wrong event type
	result, err := handler.Handle(ctx, &gitlab.PipelineEvent{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "expected *gitlab.JobEvent")
}

func TestJobEventHandlerHandleInvalidTimestamps(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()

	// Create job event with invalid timestamp format
	event := &gitlab.JobEvent{
		BuildID:         1986,
		BuildName:       "invalid-timestamp",
		BuildStage:      "test",
		BuildStatus:     "success",
		BuildStartedAt:  "invalid-time",
		BuildFinishedAt: "2021-02-23T10:05:00.000Z",
		ProjectID:       380,
		ProjectName:     testProjectName,
	}

	result, err := handler.Handle(ctx, event)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to parse started_at")
}

func TestIsCompletedJobStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"success", "success", true},
		{"failed", "failed", true},
		{"canceled", "canceled", true},
		{"skipped", "skipped", true},
		{"running", "running", false},
		{"pending", "pending", false},
		{"created", "created", false},
		{"case_insensitive_success", "SUCCESS", true},
		{"case_insensitive_failed", "FAILED", true},
		{"unknown_status", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCompletedJobStatus(tt.status)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSetJobResourceAttributes(t *testing.T) {
	event := parseJobEventFromFile(t, testDataJobSuccess)
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()

	setJobResourceAttributes(resource, event)

	attrs := resource.Attributes()
	projectID, found := attrs.Get("gitlab.project.id")
	require.True(t, found)
	require.Equal(t, "380", projectID.Str())
	projectName, found := attrs.Get("gitlab.project.name")
	require.True(t, found)
	require.Equal(t, "gitlab-org/gitlab-test", projectName.Str())
	serviceName, found := attrs.Get("service.name")
	require.True(t, found)
	require.Equal(t, testProjectName, serviceName.Str())
}

func TestCreateJobDurationMetric(t *testing.T) {
	event := parseJobEventFromFile(t, testDataJobSuccess)
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	// Parse timestamps to calculate duration
	startedAt, _ := parseGitlabTime(event.BuildStartedAt)
	finishedAt, _ := parseGitlabTime(event.BuildFinishedAt)
	duration := finishedAt.Sub(startedAt).Seconds()

	createJobDurationMetric(scopeMetrics, event, duration, "success")

	metric := scopeMetrics.Metrics().At(0)
	require.Equal(t, metricJobDuration, metric.Name())
	require.Equal(t, "s", metric.Unit())
	require.Equal(t, "Job execution duration", metric.Description())

	gauge := metric.Gauge()
	require.Equal(t, 1, gauge.DataPoints().Len())

	dp := gauge.DataPoints().At(0)
	require.Equal(t, duration, dp.DoubleValue())
	require.Positive(t, dp.Timestamp())

	attrs := dp.Attributes()
	jobID, found := attrs.Get(attrJobID)
	require.True(t, found)
	require.Equal(t, int64(1977), jobID.Int())
	jobName, found := attrs.Get(attrJobName)
	require.True(t, found)
	require.Equal(t, "test", jobName.Str())
	jobStage, found := attrs.Get(attrJobStage)
	require.True(t, found)
	require.Equal(t, "test", jobStage.Str())
	jobStatus, found := attrs.Get(attrJobStatus)
	require.True(t, found)
	require.Equal(t, "success", jobStatus.Str())
}

func TestCreateQueuedDurationMetric(t *testing.T) {
	event := parseJobEventFromFile(t, testDataJobSuccess)
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	createQueuedDurationMetric(scopeMetrics, event, 1095.588715)

	metric := scopeMetrics.Metrics().At(0)
	require.Equal(t, "gitlab.job.queued_duration", metric.Name())
	require.Equal(t, "s", metric.Unit())
	require.Equal(t, "Job queued duration", metric.Description())

	gauge := metric.Gauge()
	require.Equal(t, 1, gauge.DataPoints().Len())

	dp := gauge.DataPoints().At(0)
	require.Equal(t, 1095.588715, dp.DoubleValue())

	attrs := dp.Attributes()
	jobID, found := attrs.Get(attrJobID)
	require.True(t, found)
	require.Equal(t, int64(1977), jobID.Int())
	jobName, found := attrs.Get(attrJobName)
	require.True(t, found)
	require.Equal(t, "test", jobName.Str())
	jobStage, found := attrs.Get(attrJobStage)
	require.True(t, found)
	require.Equal(t, "test", jobStage.Str())
}

// Integration test: Test job event through the event router
func TestJobEventIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := createDefaultConfig().(*Config)
	router := newEventRouter(logger, config)

	ctx := t.Context()
	event := parseJobEventFromFile(t, testDataJobSuccess)

	result, err := router.Route(ctx, event, gitlab.EventType("Job Hook"))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Metrics)

	metrics := result.Metrics
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 2, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

// Integration test: Test job event with different time formats
func TestJobEventTimeFormats(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()

	tests := []struct {
		name        string
		startedAt   string
		finishedAt  string
		expectError bool
	}{
		{
			name:        "rfc3339_format",
			startedAt:   "2021-02-23T02:42:00Z",
			finishedAt:  "2021-02-23T02:45:30Z",
			expectError: false,
		},
		{
			name:        "gitlab_format",
			startedAt:   "2021-02-23 02:42:00 UTC",
			finishedAt:  "2021-02-23 02:45:30 UTC",
			expectError: false,
		},
		{
			name:        "invalid_format",
			startedAt:   "invalid",
			finishedAt:  "2021-02-23T02:45:30Z",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &gitlab.JobEvent{
				BuildID:         1990,
				BuildName:       "time-format-test",
				BuildStage:      "test",
				BuildStatus:     "success",
				BuildStartedAt:  tt.startedAt,
				BuildFinishedAt: tt.finishedAt,
				ProjectID:       380,
				ProjectName:     testProjectName,
			}

			result, err := handler.Handle(ctx, event)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.Metrics)
			}
		})
	}
}

// Test metric data structure validation
func TestJobEventMetricsStructure(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()
	event := parseJobEventFromFile(t, testDataJobSuccess)

	result, err := handler.Handle(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Metrics)

	metrics := result.Metrics

	// Validate resource metrics structure
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	resourceMetrics := metrics.ResourceMetrics().At(0)

	// Validate resource attributes
	resource := resourceMetrics.Resource()
	require.Positive(t, resource.Attributes().Len())

	// Validate scope metrics
	require.Equal(t, 1, resourceMetrics.ScopeMetrics().Len())
	scopeMetrics := resourceMetrics.ScopeMetrics().At(0)

	// Validate scope
	scope := scopeMetrics.Scope()
	require.NotEmpty(t, scope.Name())

	// Validate metrics count
	require.Equal(t, 2, scopeMetrics.Metrics().Len())

	// Validate each metric
	for i := 0; i < scopeMetrics.Metrics().Len(); i++ {
		metric := scopeMetrics.Metrics().At(i)
		require.NotEmpty(t, metric.Name())
		require.NotEmpty(t, metric.Unit())

		gauge := metric.Gauge()
		require.Equal(t, 1, gauge.DataPoints().Len())

		dp := gauge.DataPoints().At(0)
		require.Positive(t, dp.Timestamp())
		require.GreaterOrEqual(t, dp.DoubleValue(), 0.0)
		require.Positive(t, dp.Attributes().Len())
	}
}

// Test that incomplete jobs don't generate metrics
func TestIncompleteJobsNoMetrics(t *testing.T) {
	handler := setupJobEventHandler(t)
	ctx := t.Context()

	incompleteStatuses := []string{"running", "pending", "created", "waiting_for_resource", "preparing", "scheduled"}

	for _, status := range incompleteStatuses {
		t.Run(status, func(t *testing.T) {
			event := &gitlab.JobEvent{
				BuildID:         2000,
				BuildName:       "incomplete-job",
				BuildStage:      "test",
				BuildStatus:     status,
				BuildStartedAt:  "2021-02-23T10:01:00Z",
				BuildFinishedAt: "2021-02-23T10:05:00Z",
				ProjectID:       380,
				ProjectName:     "gitlab-org/gitlab-test",
			}

			result, err := handler.Handle(ctx, event)
			require.NoError(t, err)
			require.Nil(t, result) // Incomplete jobs should return nil
		})
	}
}

// Benchmark test for job event handling
func BenchmarkJobEventHandlerHandle(b *testing.B) {
	t := &testing.T{}
	handler := setupJobEventHandler(t)
	ctx := t.Context()
	event := parseJobEventFromFile(t, testDataJobSuccess)

	b.ResetTimer()
	//nolint:modernize // b.Loop() not available in current Go version
	for i := 0; i < b.N; i++ {
		_, _ = handler.Handle(ctx, event)
	}
}
