// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Helper function to parse a PipelineEvent from JSON for the remaining tests
func setupTestPipelineFromJSON(t *testing.T, event string) (*gitlabTracesReceiver, *gitlab.PipelineEvent, pcommon.TraceID, pcommon.SpanID) {
	receiver := setupGitlabTracesReceiver(t)

	var pipelineEvent gitlab.PipelineEvent
	err := json.Unmarshal([]byte(event), &pipelineEvent)
	require.NoError(t, err)

	// For tests that expect errors due to missing FinishedAt, don't try to create IDs that will fail
	if pipelineEvent.ObjectAttributes.FinishedAt == "" {
		return receiver, &pipelineEvent, pcommon.TraceID{}, pcommon.SpanID{}
	}

	traceID, err := newTraceID(pipelineEvent.ObjectAttributes.ID, pipelineEvent.ObjectAttributes.FinishedAt)
	require.NoError(t, err)

	spanID, err := newPipelineSpanID(pipelineEvent.ObjectAttributes.ID, pipelineEvent.ObjectAttributes.FinishedAt)
	require.NoError(t, err)

	return receiver, &pipelineEvent, traceID, spanID
}

func TestHandlePipeline(t *testing.T) {
	tests := []struct {
		name            string
		jsonEvent       string
		expectError     bool
		expectedErrText string
		spanCount       int
	}{
		{
			name:        "simple_pipeline_no_jobs",
			jsonEvent:   validPipelineWebhookEventWithoutJobs,
			expectError: false,
			spanCount:   1, // Only pipeline span
		},
		{
			name:        "pipeline_with_jobs",
			jsonEvent:   validPipelineWebhookEvent,
			expectError: false,
			spanCount:   5, // Pipeline (1) + stages (2) + jobs (2) = 5 spans
		},
		{
			name:            "invalid_pipeline_missing_finished_at",
			jsonEvent:       invalidPipelineWebhookEventMissingFinishedAt,
			expectError:     true,
			expectedErrText: "invalid finishedAt timestamp: time is empty",
			spanCount:       0,
		},
		{
			name:        "pipeline_with_minimal_fields",
			jsonEvent:   minimalValidPipelineWebhookEvent,
			expectError: false,
			spanCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, pipeline, _, _ := setupTestPipelineFromJSON(t, tt.jsonEvent)

			traces, err := receiver.handlePipeline(pipeline)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedErrText != "" {
					require.Contains(t, err.Error(), tt.expectedErrText)
				}
				return
			}

			require.NoError(t, err)

			spanCount := traces.SpanCount()
			require.Equal(t, tt.spanCount, spanCount, "unexpected span count")

			if spanCount > 0 {
				resource := traces.ResourceSpans().At(0).Resource()
				serviceName, found := resource.Attributes().Get("service.name")
				require.True(t, found, "service.name attribute should be present")
				require.Equal(t, pipeline.Project.PathWithNamespace, serviceName.Str())
			}
		})
	}
}

func TestProcessPipelineSpan(t *testing.T) {
	receiver, pipeline, traceID, spanID := setupTestPipelineFromJSON(t, validPipelineWebhookEvent)
	glPipeline := &glPipeline{pipeline}

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()

	err := receiver.processPipelineSpan(resourceSpans, glPipeline, traceID, spanID)
	require.NoError(t, err)

	// Verify the pipeline span was created
	scopeSpansCount := resourceSpans.ScopeSpans().Len()
	require.Equal(t, 1, scopeSpansCount, "should have one scope span")

	scopeSpans := resourceSpans.ScopeSpans().At(0)
	require.Equal(t, 1, scopeSpans.Spans().Len(), "should have one span")

	span := scopeSpans.Spans().At(0)
	require.Equal(t, traceID, span.TraceID(), "trace IDs should match")
	require.Equal(t, spanID, span.SpanID(), "span IDs should match")
	require.Equal(t, pipeline.ObjectAttributes.Name, span.Name(), "span name should match pipeline name")
}

func TestProcessStageSpans(t *testing.T) {
	receiver, pipeline, traceID, parentSpanID := setupTestPipelineFromJSON(t, validPipelineWebhookEvent)
	glPipeline := &glPipeline{pipeline}

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()

	stages, err := receiver.processStageSpans(resourceSpans, glPipeline, traceID, parentSpanID)
	require.NoError(t, err)
	require.Len(t, stages, 2, "should have two stages")

	// Verify the stage spans were created
	scopeSpansCount := resourceSpans.ScopeSpans().Len()
	for i := range scopeSpansCount {
		scopeSpans := resourceSpans.ScopeSpans().At(i)
		require.Equal(t, 1, scopeSpans.Spans().Len(), "each scope span should have one span")

		span := scopeSpans.Spans().At(0)
		require.Equal(t, traceID, span.TraceID(), "trace IDs should match")
		require.Equal(t, parentSpanID, span.ParentSpanID(), "parent span IDs should match")
	}
}

func TestProcessJobSpans(t *testing.T) {
	receiver, pipeline, traceID, _ := setupTestPipelineFromJSON(t, validPipelineWebhookEvent)
	glPipeline := &glPipeline{pipeline}

	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()

	stages, err := receiver.newStages(glPipeline)
	require.NoError(t, err)

	err = receiver.processJobSpans(resourceSpans, glPipeline, traceID, stages)
	require.NoError(t, err)

	// Verify the job spans were created
	scopeSpansCount := resourceSpans.ScopeSpans().Len()
	for i := range scopeSpansCount {
		scopeSpans := resourceSpans.ScopeSpans().At(i)
		require.Equal(t, 1, scopeSpans.Spans().Len(), "each scope span should have one span")

		span := scopeSpans.Spans().At(0)
		require.Equal(t, traceID, span.TraceID(), "trace IDs should match")

		// Get the job's stage to check the parent span ID
		job := pipeline.Builds[i]
		expectedParentSpanID, err := newStageSpanID(pipeline.ObjectAttributes.ID, job.Stage, stages[job.Stage].StartedAt)
		require.NoError(t, err)

		require.Equal(t, expectedParentSpanID, span.ParentSpanID(), "parent span IDs should match stage span ID")
	}
}

func TestNewTraceID(t *testing.T) {
	tests := []struct {
		name       string
		id         int64
		finishedAt string
		expectErr  bool
	}{
		{
			name:       "valid input",
			id:         12345,
			finishedAt: "2022-01-01 12:00:00 UTC",
			expectErr:  false,
		},
		{
			name:       "invalid timestamp",
			id:         12345,
			finishedAt: "invalid timestamp",
			expectErr:  true,
		},
		{
			name:       "empty timestamp",
			id:         12345,
			finishedAt: "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID, err := newTraceID(tt.id, tt.finishedAt)
			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, pcommon.TraceID{}, traceID)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, pcommon.TraceID{}, traceID)
			}
		})
	}
}

func TestNewPipelineSpanID(t *testing.T) {
	tests := []struct {
		name       string
		id         int64
		finishedAt string
		expectErr  bool
	}{
		{
			name:       "valid input",
			id:         12345,
			finishedAt: "2022-01-01 12:00:00 UTC",
			expectErr:  false,
		},
		{
			name:       "invalid timestamp",
			id:         12345,
			finishedAt: "invalid timestamp",
			expectErr:  true,
		},
		{
			name:       "empty timestamp",
			id:         12345,
			finishedAt: "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanID, err := newPipelineSpanID(tt.id, tt.finishedAt)
			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, pcommon.SpanID{}, spanID)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, pcommon.SpanID{}, spanID)
			}
		})
	}
}

func TestNewStageSpanID(t *testing.T) {
	tests := []struct {
		name      string
		id        int64
		stage     string
		startedAt string
		expectErr bool
	}{
		{
			name:      "valid input",
			id:        12345,
			stage:     "build",
			startedAt: "2022-01-01 12:00:00 UTC",
			expectErr: false,
		},
		{
			name:      "invalid timestamp",
			id:        12345,
			stage:     "build",
			startedAt: "invalid timestamp",
			expectErr: true,
		},
		{
			name:      "empty stage",
			id:        12345,
			stage:     "",
			startedAt: "2022-01-01 12:00:00 UTC",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanID, err := newStageSpanID(tt.id, tt.stage, tt.startedAt)
			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, pcommon.SpanID{}, spanID)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, pcommon.SpanID{}, spanID)
			}
		})
	}
}

func TestNewJobSpanID(t *testing.T) {
	tests := []struct {
		name      string
		id        int64
		startedAt string
		expectErr bool
	}{
		{
			name:      "valid input",
			id:        12345,
			startedAt: "2022-01-01 12:00:00 UTC",
			expectErr: false,
		},
		{
			name:      "invalid timestamp",
			id:        12345,
			startedAt: "invalid timestamp",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanID, err := newJobSpanID(tt.id, tt.startedAt)
			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, pcommon.SpanID{}, spanID)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, pcommon.SpanID{}, spanID)
			}
		})
	}
}

func TestParseGitlabTime(t *testing.T) {
	tests := []struct {
		name        string
		timeStr     string
		expectError bool
	}{
		{
			name:        "valid UTC time",
			timeStr:     "2022-01-01 12:00:00 UTC",
			expectError: false,
		},
		{
			name:        "valid RFC3339 time",
			timeStr:     "2022-01-01T12:00:00Z",
			expectError: false,
		},
		{
			name:        "valid RFC3339 time with milliseconds",
			timeStr:     "2022-01-01T12:00:00.123Z",
			expectError: false,
		},
		{
			name:        "empty time string",
			timeStr:     "",
			expectError: true,
		},
		{
			name:        "null string",
			timeStr:     "null",
			expectError: true,
		},
		{
			name:        "invalid time format",
			timeStr:     "invalid-timestamp",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			time, err := parseGitlabTime(tt.timeStr)
			if tt.expectError {
				require.Error(t, err, "expected error for time string: %s", tt.timeStr)
				require.True(t, time.IsZero(), "expected zero time value")
			} else {
				require.NoError(t, err, "did not expect error for time string: %s", tt.timeStr)
				require.False(t, time.IsZero(), "expected non-zero time value")
			}
		})
	}
}

func TestIncludeUserAttributes(t *testing.T) {
	tests := []struct {
		name                  string
		includeUserAttributes bool
		expectUserAttrs       bool
	}{
		{
			name:                  "user details excluded by default",
			includeUserAttributes: false,
			expectUserAttrs:       false,
		},
		{
			name:                  "user details included when enabled",
			includeUserAttributes: true,
			expectUserAttrs:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := setupGitlabTracesReceiver(t)
			receiver.cfg.WebHook.IncludeUserAttributes = tt.includeUserAttributes

			var pipelineEvent gitlab.PipelineEvent
			err := json.Unmarshal([]byte(validPipelineWebhookEvent), &pipelineEvent)
			require.NoError(t, err)

			attrs := pcommon.NewMap()
			receiver.setResourceAttributes(attrs, &pipelineEvent)

			_, hasAuthorName := attrs.Get(AttributeVCSRefHeadRevisionAuthorName)
			_, hasAuthorEmail := attrs.Get(AttributeVCSRefHeadRevisionAuthorEmail)
			_, hasCommitMessage := attrs.Get(AttributeVCSRefHeadRevisionMessage)
			_, hasActorID := attrs.Get(AttributeCICDPipelineRunActorID)
			_, hasActorUsername := attrs.Get(AttributeGitLabPipelineRunActorUsername)
			_, hasActorName := attrs.Get(AttributeCICDPipelineRunActorName)

			if tt.expectUserAttrs {
				require.True(t, hasAuthorName, "expected author name attribute")
				require.True(t, hasAuthorEmail, "expected author email attribute")
				require.True(t, hasCommitMessage, "expected commit message attribute")
				require.True(t, hasActorID, "expected actor ID attribute")
				require.True(t, hasActorUsername, "expected actor username attribute")
				require.True(t, hasActorName, "expected actor name attribute")
			} else {
				require.False(t, hasAuthorName, "unexpected author name attribute")
				require.False(t, hasAuthorEmail, "unexpected author email attribute")
				require.False(t, hasCommitMessage, "unexpected commit message attribute")
				require.False(t, hasActorID, "unexpected actor ID attribute")
				require.False(t, hasActorUsername, "unexpected actor username attribute")
				require.False(t, hasActorName, "unexpected actor name attribute")
			}

			// Non-sensitive attributes should always be present
			_, hasServiceName := attrs.Get("service.name")
			require.True(t, hasServiceName, "service.name should always be present")
		})
	}
}

func TestSetAttributes(t *testing.T) {
	receiver := setupGitlabTracesReceiver(t)
	receiver.cfg.WebHook.IncludeUserAttributes = true

	var pipelineEvent gitlab.PipelineEvent
	err := json.Unmarshal([]byte(validPipelineWebhookEvent), &pipelineEvent)
	require.NoError(t, err)

	attrs := pcommon.NewMap()
	receiver.setResourceAttributes(attrs, &pipelineEvent)

	// VCS
	vcsProvider, _ := attrs.Get("vcs.provider.name")
	require.Equal(t, "gitlab", vcsProvider.Str())

	repoName, _ := attrs.Get("vcs.repository.name")
	require.Equal(t, "my-project", repoName.Str())

	repoURL, _ := attrs.Get("vcs.repository.url.full")
	require.Equal(t, "https://gitlab.example.com/test/project", repoURL.Str())

	visibility, _ := attrs.Get(AttributeVCSRepositoryVisibility)
	require.Equal(t, "private", visibility.Str())

	namespace, _ := attrs.Get(AttributeGitLabProjectNamespace)
	require.Equal(t, "test", namespace.Str())

	defaultBranch, _ := attrs.Get(AttributeVCSRepositoryRefDefault)
	require.Equal(t, "main", defaultBranch.Str())

	// Pipeline
	pipelineAttrs := pcommon.NewMap()
	glPipeline := &glPipeline{&pipelineEvent}
	glPipeline.setAttributes(pipelineAttrs)

	pipelineSource, _ := pipelineAttrs.Get(AttributeGitLabPipelineSource)
	require.Equal(t, "push", pipelineSource.Str())

	// Job
	if len(pipelineEvent.Builds) > 0 {
		job := &glPipelineJob{
			event:  &glJobEvent{},
			jobURL: "https://example.com/job/1",
		}
		buildData, _ := json.Marshal(pipelineEvent.Builds[0])
		err := json.Unmarshal(buildData, job.event)
		require.NoError(t, err)

		jobAttrs := pcommon.NewMap()
		job.setAttributes(jobAttrs)

		queuedDuration, _ := jobAttrs.Get(AttributeGitLabJobQueuedDuration)
		require.Equal(t, 60.5, queuedDuration.Double())

		allowFailure, _ := jobAttrs.Get(AttributeGitLabJobAllowFailure)
		require.False(t, allowFailure.Bool())

		runnerType, _ := jobAttrs.Get(AttributeCICDWorkerType)
		require.Equal(t, "instance_type", runnerType.Str())

		isShared, _ := jobAttrs.Get(AttributeCICDWorkerShared)
		require.True(t, isShared.Bool())

		tags, _ := jobAttrs.Get(AttributeCICDWorkerTags)
		require.Equal(t, 2, tags.Slice().Len())
		require.Equal(t, "docker", tags.Slice().At(0).Str())
		require.Equal(t, "linux", tags.Slice().At(1).Str())
	}
}

func TestMultiPipeline(t *testing.T) {
	var pipelineEvent gitlab.PipelineEvent
	err := json.Unmarshal([]byte(validMultiPipelineWebhookEvent), &pipelineEvent)
	require.NoError(t, err)

	pipelineAttrs := pcommon.NewMap()
	glPipeline := &glPipeline{&pipelineEvent}
	glPipeline.setAttributes(pipelineAttrs)

	// Pipeline source
	pipelineSource, _ := pipelineAttrs.Get(AttributeGitLabPipelineSource)
	require.Equal(t, "parent_pipeline", pipelineSource.Str())

	// Parent pipeline
	parentProjectID, _ := pipelineAttrs.Get(AttributeGitLabPipelineSourcePipelineProjectID)
	require.Equal(t, int64(123), parentProjectID.Int())

	parentPipelineID, _ := pipelineAttrs.Get(AttributeGitLabPipelineSourcePipelineID)
	require.Equal(t, int64(1), parentPipelineID.Int())

	parentJobID, _ := pipelineAttrs.Get(AttributeGitLabPipelineSourcePipelineJobID)
	require.Equal(t, int64(99), parentJobID.Int())

	parentProjectNamespace, _ := pipelineAttrs.Get(AttributeGitLabPipelineSourcePipelineProjectNamespace)
	require.Equal(t, "test/parent-project", parentProjectNamespace.Str())

	parentProjectURL, _ := pipelineAttrs.Get(AttributeGitLabPipelineSourcePipelineProjectURL)
	require.Equal(t, "https://gitlab.example.com/test/parent-project", parentProjectURL.Str())
}

func TestPipelineWithMissingOptionalFields(t *testing.T) {
	receiver, pipeline, _, _ := setupTestPipelineFromJSON(t, minimalValidPipelineWebhookEvent)
	receiver.cfg.WebHook.IncludeUserAttributes = true

	traces, err := receiver.handlePipeline(pipeline)
	require.NoError(t, err)

	require.Equal(t, 1, traces.SpanCount())

	resource := traces.ResourceSpans().At(0).Resource()
	attrs := resource.Attributes()

	// Required fields should be present
	serviceName, found := attrs.Get("service.name")
	require.True(t, found)
	require.Equal(t, "test/project", serviceName.Str())

	// Optional fields should be present but may be empty
	pipelineName, found := attrs.Get("cicd.pipeline.name")
	require.True(t, found)
	require.Empty(t, pipelineName.Str())

	repoName, found := attrs.Get("vcs.repository.name")
	require.True(t, found)
	require.Empty(t, repoName.Str())

	// Timestamp should not be present since it's nil
	_, found = attrs.Get(AttributeVCSRefHeadRevisionTimestamp)
	require.False(t, found)

	// User attributes with include_user_attributes=true but user is nil
	_, found = attrs.Get(AttributeCICDPipelineRunActorID)
	require.False(t, found)

	// Author fields should be empty strings
	authorName, found := attrs.Get(AttributeVCSRefHeadRevisionAuthorName)
	require.True(t, found)
	require.Empty(t, authorName.Str())

	authorEmail, found := attrs.Get(AttributeVCSRefHeadRevisionAuthorEmail)
	require.True(t, found)
	require.Empty(t, authorEmail.Str())

	// Merge request ID should not create attributes when 0
	_, found = attrs.Get("vcs.change.id")
	require.False(t, found)
}

func TestEnvironmentAttributes(t *testing.T) {
	var pipelineEvent gitlab.PipelineEvent
	err := json.Unmarshal([]byte(validPipelineWithEnvironmentEvent), &pipelineEvent)
	require.NoError(t, err)

	stagingJob := &glPipelineJob{
		event:  &glJobEvent{},
		jobURL: "https://example.com/job/10",
	}

	buildData, _ := json.Marshal(pipelineEvent.Builds[0])
	err = json.Unmarshal(buildData, stagingJob.event)
	require.NoError(t, err)

	stagingAttrs := pcommon.NewMap()
	stagingJob.setAttributes(stagingAttrs)

	envName, _ := stagingAttrs.Get(AttributeCICDTaskEnvironmentName)
	require.Equal(t, "staging", envName.Str())

	deploymentTier, _ := stagingAttrs.Get(AttributeGitLabEnvironmentDeploymentTier)
	require.Equal(t, "staging", deploymentTier.Str())

	envAction, _ := stagingAttrs.Get(AttributeGitLabEnvironmentAction)
	require.Equal(t, "start", envAction.Str())

	prodJob := &glPipelineJob{
		event:  &glJobEvent{},
		jobURL: "https://example.com/job/11",
	}
	buildData, _ = json.Marshal(pipelineEvent.Builds[1])
	err = json.Unmarshal(buildData, prodJob.event)
	require.NoError(t, err)

	prodAttrs := pcommon.NewMap()
	prodJob.setAttributes(prodAttrs)

	envName, _ = prodAttrs.Get(AttributeCICDTaskEnvironmentName)
	require.Equal(t, "production", envName.Str())

	deploymentTier, _ = prodAttrs.Get(AttributeGitLabEnvironmentDeploymentTier)
	require.Equal(t, "production", deploymentTier.Str())

	failureReason, _ := prodAttrs.Get(AttributeGitLabJobFailureReason)
	require.Equal(t, "script_failure", failureReason.Str())

	allowFailure, _ := prodAttrs.Get(AttributeGitLabJobAllowFailure)
	require.True(t, allowFailure.Bool())
}

func TestRunnerAttributes(t *testing.T) {
	tests := []struct {
		name           string
		runnerData     string
		expectedType   string
		expectedShared bool
		expectedTags   []string
	}{
		{
			name:           "instance runner with tags",
			runnerData:     `{"id":100,"description":"shared-runner","active":true,"is_shared":true,"runner_type":"instance_type","tags":["docker","linux","amd64"]}`,
			expectedType:   "instance_type",
			expectedShared: true,
			expectedTags:   []string{"docker", "linux", "amd64"},
		},
		{
			name:           "group runner",
			runnerData:     `{"id":200,"description":"group-runner","active":true,"is_shared":false,"runner_type":"group_type","tags":["kubernetes"]}`,
			expectedType:   "group_type",
			expectedShared: false,
			expectedTags:   []string{"kubernetes"},
		},
		{
			name:           "project runner without tags",
			runnerData:     `{"id":300,"description":"project-runner","active":true,"is_shared":false,"runner_type":"project_type","tags":[]}`,
			expectedType:   "project_type",
			expectedShared: false,
			expectedTags:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runner gitlab.PipelineEventBuildRunner
			err := json.Unmarshal([]byte(tt.runnerData), &runner)
			require.NoError(t, err)

			job := &glPipelineJob{
				event: &glJobEvent{
					ID:     1,
					Name:   "test-job",
					Runner: runner,
				},
				jobURL: "https://example.com/job/1",
			}

			attrs := pcommon.NewMap()
			job.setAttributes(attrs)

			runnerType, _ := attrs.Get(AttributeCICDWorkerType)
			require.Equal(t, tt.expectedType, runnerType.Str())

			isShared, _ := attrs.Get(AttributeCICDWorkerShared)
			require.Equal(t, tt.expectedShared, isShared.Bool())

			if len(tt.expectedTags) > 0 {
				tags, found := attrs.Get(AttributeCICDWorkerTags)
				require.True(t, found, "expected runner tags")
				require.Equal(t, len(tt.expectedTags), tags.Slice().Len())
				for i, expectedTag := range tt.expectedTags {
					require.Equal(t, expectedTag, tags.Slice().At(i).Str())
				}
			} else {
				_, found := attrs.Get(AttributeCICDWorkerTags)
				require.False(t, found, "unexpected runner tags for runner without tags")
			}
		})
	}
}
