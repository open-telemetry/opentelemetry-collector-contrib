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
	for i := 0; i < scopeSpansCount; i++ {
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
	for i := 0; i < scopeSpansCount; i++ {
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
		id         int
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
		id         int
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
		id        int
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
		id        int
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
