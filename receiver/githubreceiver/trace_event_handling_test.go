// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
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
				settings: receivertest.NewNopSettings(),
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
