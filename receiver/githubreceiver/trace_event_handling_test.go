// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"testing"
	"time"

	"github.com/google/go-github/v70/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

func TestHandleWorkflowRun(t *testing.T) {
	// Helper function to create a test workflow run event
	createTestWorkflowRunEvent := func(id int64, runAttempt int, name, conclusion string, startTime, updateTime time.Time) *github.WorkflowRunEvent {
		return &github.WorkflowRunEvent{
			WorkflowRun: &github.WorkflowRun{
				ID:           &id,
				Name:         &name,
				RunAttempt:   &runAttempt,
				RunStartedAt: &github.Timestamp{Time: startTime},
				UpdatedAt:    &github.Timestamp{Time: updateTime},
				Conclusion:   &conclusion,
			},
			Repo: &github.Repository{
				Name:          github.Ptr("test-repo"),
				Organization:  &github.Organization{Login: github.Ptr("test-org")},
				DefaultBranch: github.Ptr("main"),
				CustomProperties: map[string]any{
					"service_name": "test-service",
				},
			},
			Workflow: &github.Workflow{
				Name: github.Ptr("test-workflow"),
				Path: github.Ptr(".github/workflows/test.yml"),
			},
		}
	}

	tests := []struct {
		name     string
		event    *github.WorkflowRunEvent
		wantErr  bool
		validate func(t *testing.T, traces ptrace.Traces)
	}{
		{
			name: "successful workflow run",
			event: createTestWorkflowRunEvent(
				123,
				1,
				"Test Workflow",
				"success",
				time.Now().Add(-time.Hour),
				time.Now(),
			),
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				require.Equal(t, 1, traces.ResourceSpans().Len())

				rs := traces.ResourceSpans().At(0)
				require.Equal(t, 1, rs.ScopeSpans().Len())

				spans := rs.ScopeSpans().At(0).Spans()
				require.Equal(t, 1, spans.Len())

				span := spans.At(0)
				require.Equal(t, "Test Workflow", span.Name())
				require.Equal(t, ptrace.SpanKindServer, span.Kind())
				require.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
				require.Equal(t, "success", span.Status().Message())
			},
		},
		{
			name: "failed workflow run",
			event: createTestWorkflowRunEvent(
				124,
				1,
				"Test Workflow",
				"failure",
				time.Now().Add(-time.Hour),
				time.Now(),
			),
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				spans := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
				span := spans.At(0)
				require.Equal(t, ptrace.StatusCodeError, span.Status().Code())
				require.Equal(t, "failure", span.Status().Message())
			},
		},
		{
			name: "workflow run with retry",
			event: func() *github.WorkflowRunEvent {
				e := createTestWorkflowRunEvent(
					125,
					2,
					"Test Workflow",
					"success",
					time.Now().Add(-time.Hour),
					time.Now(),
				)
				previousURL := "https://api.github.com/repos/test-org/test-repo/actions/runs/125/attempts/1"
				e.WorkflowRun.PreviousAttemptURL = &previousURL
				return e
			}(),
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				spans := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
				span := spans.At(0)
				require.Equal(t, 1, span.Links().Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new receiver with a test logger
			logger := zap.NewNop()
			receiver := &githubTracesReceiver{
				logger:   logger,
				cfg:      createDefaultConfig().(*Config),
				settings: receivertest.NewNopSettings(metadata.Type),
			}

			// Handle the workflow run event
			traces, err := receiver.handleWorkflowRun(tt.event)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.validate(t, traces)
		})
	}
}

func TestNewParentSpanID(t *testing.T) {
	tests := []struct {
		name       string
		runID      int64
		runAttempt int
		wantError  bool
	}{
		{
			name:       "basic span ID generation",
			runID:      12345,
			runAttempt: 1,
			wantError:  false,
		},
		{
			name:       "different run ID",
			runID:      54321,
			runAttempt: 1,
			wantError:  false,
		},
		{
			name:       "different attempt",
			runID:      12345,
			runAttempt: 2,
			wantError:  false,
		},
		{
			name:       "zero values",
			runID:      0,
			runAttempt: 0,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call to get span ID
			spanID1, err1 := newParentSpanID(tt.runID, tt.runAttempt)

			if tt.wantError {
				require.Error(t, err1)
				return
			}
			require.NoError(t, err1)

			// Verify span ID is not empty
			require.NotEqual(t, pcommon.SpanID{}, spanID1, "span ID should not be empty")

			// Verify consistent results for same input
			spanID2, err2 := newParentSpanID(tt.runID, tt.runAttempt)
			require.NoError(t, err2)
			require.Equal(t, spanID1, spanID2, "same inputs should generate same span ID")

			// Verify different inputs generate different span IDs
			differentSpanID, err3 := newParentSpanID(tt.runID+1, tt.runAttempt)
			require.NoError(t, err3)
			require.NotEqual(t, spanID1, differentSpanID, "different inputs should generate different span IDs")
		})
	}
}

func TestNewParentSpanID_Consistency(t *testing.T) {
	// Test that generates the same span ID for same inputs across multiple calls
	runID := int64(12345)
	runAttempt := 1

	spanID1, err1 := newParentSpanID(runID, runAttempt)
	require.NoError(t, err1)

	for i := 0; i < 5; i++ {
		spanID2, err2 := newParentSpanID(runID, runAttempt)
		require.NoError(t, err2)
		require.Equal(t, spanID1, spanID2, "span ID should be consistent across multiple calls")
	}
}

func TestNewUniqueSteps(t *testing.T) {
	tests := []struct {
		name     string
		steps    []*github.TaskStep
		expected []string
	}{
		{
			name:     "nil steps",
			steps:    nil,
			expected: nil,
		},
		{
			name:     "empty steps",
			steps:    []*github.TaskStep{},
			expected: nil,
		},
		{
			name: "no duplicate steps",
			steps: []*github.TaskStep{
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Test")},
				{Name: github.Ptr("Deploy")},
			},
			expected: []string{"Build", "Test", "Deploy"},
		},
		{
			name: "with duplicate steps",
			steps: []*github.TaskStep{
				{Name: github.Ptr("Setup")},
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Test")},
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Test")},
				{Name: github.Ptr("Deploy")},
			},
			expected: []string{"Setup", "Build", "Test", "Build-1", "Test-1", "Deploy"},
		},
		{
			name: "multiple duplicates of same step",
			steps: []*github.TaskStep{
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Build")},
				{Name: github.Ptr("Build")},
			},
			expected: []string{"Build", "Build-1", "Build-2", "Build-3"},
		},
		{
			name: "with empty step names",
			steps: []*github.TaskStep{
				{Name: github.Ptr("")},
				{Name: github.Ptr("")},
				{Name: github.Ptr("Build")},
			},
			expected: []string{"", "-1", "Build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newUniqueSteps(tt.steps)

			// Check length matches
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}

			// Check contents match
			for i := 0; i < len(result); i++ {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCreateStepSpans(t *testing.T) {
	// Helper function to create a workflow job event with steps
	createTestWorkflowJobEvent := func(steps []*github.TaskStep) *github.WorkflowJobEvent {
		return &github.WorkflowJobEvent{
			WorkflowJob: &github.WorkflowJob{
				ID:         github.Ptr(int64(123)),
				RunID:      github.Ptr(int64(456)),
				RunAttempt: github.Ptr(int64(1)),
				Name:       github.Ptr("Test Job"),
				Steps:      steps,
			},
		}
	}

	// Helper function to create a timestamp
	now := time.Now()
	createTimestamp := func(offsetMinutes int) *github.Timestamp {
		return &github.Timestamp{Time: now.Add(time.Duration(offsetMinutes) * time.Minute)}
	}

	tests := []struct {
		name       string
		event      *github.WorkflowJobEvent
		wantErr    bool
		validateFn func(t *testing.T, spans ptrace.SpanSlice)
	}{
		{
			name: "single step",
			event: createTestWorkflowJobEvent([]*github.TaskStep{
				{
					Name:        github.Ptr("Build"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("success"),
					Number:      github.Ptr(int64(1)),
					StartedAt:   createTimestamp(0),
					CompletedAt: createTimestamp(5),
				},
			}),
			wantErr: false,
			validateFn: func(t *testing.T, spans ptrace.SpanSlice) {
				require.Equal(t, 1, spans.Len())
				span := spans.At(0)
				require.Equal(t, "Build", span.Name())
				require.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
			},
		},
		{
			name: "multiple steps with different states",
			event: createTestWorkflowJobEvent([]*github.TaskStep{
				{
					Name:        github.Ptr("Setup"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("success"),
					Number:      github.Ptr(int64(1)),
					StartedAt:   createTimestamp(0),
					CompletedAt: createTimestamp(2),
				},
				{
					Name:        github.Ptr("Build"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("failure"),
					Number:      github.Ptr(int64(2)),
					StartedAt:   createTimestamp(2),
					CompletedAt: createTimestamp(5),
				},
				{
					Name:        github.Ptr("Test"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("skipped"),
					Number:      github.Ptr(int64(3)),
					StartedAt:   createTimestamp(5),
					CompletedAt: createTimestamp(6),
				},
			}),
			wantErr: false,
			validateFn: func(t *testing.T, spans ptrace.SpanSlice) {
				require.Equal(t, 3, spans.Len())

				// Setup step
				setup := spans.At(0)
				require.Equal(t, "Setup", setup.Name())
				require.Equal(t, ptrace.StatusCodeOk, setup.Status().Code())

				// Build step
				build := spans.At(1)
				require.Equal(t, "Build", build.Name())
				require.Equal(t, ptrace.StatusCodeError, build.Status().Code())

				// Test step
				test := spans.At(2)
				require.Equal(t, "Test", test.Name())
				require.Equal(t, ptrace.StatusCodeUnset, test.Status().Code())
			},
		},
		{
			name: "duplicate step names",
			event: createTestWorkflowJobEvent([]*github.TaskStep{
				{
					Name:        github.Ptr("Build"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("success"),
					Number:      github.Ptr(int64(1)),
					StartedAt:   createTimestamp(0),
					CompletedAt: createTimestamp(2),
				},
				{
					Name:        github.Ptr("Build"),
					Status:      github.Ptr("completed"),
					Conclusion:  github.Ptr("success"),
					Number:      github.Ptr(int64(2)),
					StartedAt:   createTimestamp(2),
					CompletedAt: createTimestamp(4),
				},
			}),
			wantErr: false,
			validateFn: func(t *testing.T, spans ptrace.SpanSlice) {
				require.Equal(t, 2, spans.Len())
				require.Equal(t, "Build", spans.At(0).Name())
				require.Equal(t, "Build-1", spans.At(1).Name())
			},
		},
		{
			name:    "no steps",
			event:   createTestWorkflowJobEvent([]*github.TaskStep{}),
			wantErr: false,
			validateFn: func(t *testing.T, spans ptrace.SpanSlice) {
				require.Equal(t, 0, spans.Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new receiver with a test logger
			logger := zap.NewNop()
			receiver := &githubTracesReceiver{
				logger:   logger,
				cfg:      createDefaultConfig().(*Config),
				settings: receivertest.NewNopSettings(metadata.Type),
			}

			// Create traces and resource spans
			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans().AppendEmpty()

			// Generate a trace ID and parent span ID for testing
			traceID, err := newTraceID(tt.event.GetWorkflowJob().GetRunID(), int(tt.event.GetWorkflowJob().GetRunAttempt()))
			require.NoError(t, err)
			parentSpanID, err := newParentSpanID(tt.event.GetWorkflowJob().GetID(), int(tt.event.GetWorkflowJob().GetRunAttempt()))
			require.NoError(t, err)

			// Call createStepSpans
			err = receiver.createStepSpans(resourceSpans, tt.event, traceID, parentSpanID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			// Get all spans from all scope spans
			var allSpans []ptrace.Span
			for i := 0; i < resourceSpans.ScopeSpans().Len(); i++ {
				scopeSpans := resourceSpans.ScopeSpans().At(i)
				spans := scopeSpans.Spans()
				for j := 0; j < spans.Len(); j++ {
					allSpans = append(allSpans, spans.At(j))
				}
			}

			// Convert to SpanSlice for validation
			spanSlice := ptrace.NewSpanSlice()
			for _, span := range allSpans {
				spanCopy := spanSlice.AppendEmpty()
				span.CopyTo(spanCopy)
			}

			// Run validation
			tt.validateFn(t, spanSlice)
		})
	}
}

func TestNewStepSpanID(t *testing.T) {
	tests := []struct {
		name       string
		runID      int64
		runAttempt int
		jobName    string
		stepName   string
		number     int
		wantError  bool
	}{
		{
			name:       "basic step span ID",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job",
			stepName:   "build",
			number:     1,
			wantError:  false,
		},
		{
			name:       "different run ID",
			runID:      54321,
			runAttempt: 1,
			jobName:    "test-job",
			stepName:   "build",
			number:     1,
			wantError:  false,
		},
		{
			name:       "different attempt",
			runID:      12345,
			runAttempt: 2,
			jobName:    "test-job",
			stepName:   "build",
			number:     1,
			wantError:  false,
		},
		{
			name:       "different job name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "other-job",
			stepName:   "build",
			number:     1,
			wantError:  false,
		},
		{
			name:       "different step name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job",
			stepName:   "test",
			number:     1,
			wantError:  false,
		},
		{
			name:       "different number",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job",
			stepName:   "build",
			number:     2,
			wantError:  false,
		},
		{
			name:       "zero values",
			runID:      0,
			runAttempt: 0,
			jobName:    "",
			stepName:   "",
			number:     0,
			wantError:  false,
		},
		{
			name:       "with special characters in names",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job!@#$%^&*()",
			stepName:   "build step with spaces",
			number:     1,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call to get span ID
			spanID1, err1 := newStepSpanID(tt.runID, tt.runAttempt, tt.jobName, tt.stepName, tt.number)

			if tt.wantError {
				require.Error(t, err1)
				return
			}
			require.NoError(t, err1)

			// Verify span ID is not empty
			require.NotEqual(t, pcommon.SpanID{}, spanID1, "span ID should not be empty")

			// Verify consistent results for same input
			spanID2, err2 := newStepSpanID(tt.runID, tt.runAttempt, tt.jobName, tt.stepName, tt.number)
			require.NoError(t, err2)
			require.Equal(t, spanID1, spanID2, "same inputs should generate same span ID")

			// Verify different inputs generate different span IDs
			differentSpanID, err3 := newStepSpanID(tt.runID+1, tt.runAttempt, tt.jobName, tt.stepName, tt.number)
			require.NoError(t, err3)
			require.NotEqual(t, spanID1, differentSpanID, "different inputs should generate different span IDs")
		})
	}
}

func TestNewStepSpanID_Consistency(t *testing.T) {
	// Test that generates the same span ID for same inputs across multiple calls
	runID := int64(12345)
	runAttempt := 1
	jobName := "test-job"
	stepName := "build"
	number := 1

	spanID1, err1 := newStepSpanID(runID, runAttempt, jobName, stepName, number)
	require.NoError(t, err1)

	for i := 0; i < 5; i++ {
		spanID2, err2 := newStepSpanID(runID, runAttempt, jobName, stepName, number)
		require.NoError(t, err2)
		require.Equal(t, spanID1, spanID2, "span ID should be consistent across multiple calls")
	}
}

func TestNewJobSpanID(t *testing.T) {
	tests := []struct {
		name       string
		runID      int64
		runAttempt int
		jobName    string
		wantError  bool
	}{
		{
			name:       "basic job span ID",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job",
			wantError:  false,
		},
		{
			name:       "different run ID",
			runID:      54321,
			runAttempt: 1,
			jobName:    "test-job",
			wantError:  false,
		},
		{
			name:       "different attempt",
			runID:      12345,
			runAttempt: 2,
			jobName:    "test-job",
			wantError:  false,
		},
		{
			name:       "different job name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "other-job",
			wantError:  false,
		},
		{
			name:       "zero values",
			runID:      0,
			runAttempt: 0,
			jobName:    "",
			wantError:  false,
		},
		{
			name:       "with special characters in job name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test-job!@#$%^&*()",
			wantError:  false,
		},
		{
			name:       "with spaces in job name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "test job with spaces",
			wantError:  false,
		},
		{
			name:       "with unicode in job name",
			runID:      12345,
			runAttempt: 1,
			jobName:    "测试工作",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call to get span ID
			spanID1, err1 := newJobSpanID(tt.runID, tt.runAttempt, tt.jobName)

			if tt.wantError {
				require.Error(t, err1)
				return
			}
			require.NoError(t, err1)

			// Verify span ID is not empty
			require.NotEqual(t, pcommon.SpanID{}, spanID1, "span ID should not be empty")

			// Verify consistent results for same input
			spanID2, err2 := newJobSpanID(tt.runID, tt.runAttempt, tt.jobName)
			require.NoError(t, err2)
			require.Equal(t, spanID1, spanID2, "same inputs should generate same span ID")

			// Verify different inputs generate different span IDs
			differentSpanID, err3 := newJobSpanID(tt.runID+1, tt.runAttempt, tt.jobName)
			require.NoError(t, err3)
			require.NotEqual(t, spanID1, differentSpanID, "different inputs should generate different span IDs")
		})
	}
}

func TestNewJobSpanID_Consistency(t *testing.T) {
	// Test that generates the same span ID for same inputs across multiple calls
	runID := int64(12345)
	runAttempt := 1
	jobName := "test-job"

	spanID1, err1 := newJobSpanID(runID, runAttempt, jobName)
	require.NoError(t, err1)

	for i := 0; i < 5; i++ {
		spanID2, err2 := newJobSpanID(runID, runAttempt, jobName)
		require.NoError(t, err2)
		require.Equal(t, spanID1, spanID2, "span ID should be consistent across multiple calls")
	}
}
