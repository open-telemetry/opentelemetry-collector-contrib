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

func TestGlPipelineSetSpanData(t *testing.T) {
	tests := []struct {
		name        string
		pipelineStr string
		expectError bool
	}{
		{
			name: "valid pipeline with name",
			pipelineStr: `{
				"object_attributes": {
					"id": 123,
					"status": "success",
					"created_at": "2022-01-01 12:00:00 UTC",
					"finished_at": "2022-01-01 13:00:00 UTC",
					"name": "Test Pipeline"
				},
				"project": {
					"path_with_namespace": "test/project"
				}
			}`,
			expectError: false,
		},
		{
			name: "valid pipeline with commit title instead of name",
			pipelineStr: `{
				"object_attributes": {
					"id": 123,
					"status": "success",
					"created_at": "2022-01-01 12:00:00 UTC",
					"finished_at": "2022-01-01 13:00:00 UTC"
				},
				"project": {
					"path_with_namespace": "test/project"
				},
				"commit": {
					"title": "Test Commit"
				}
			}`,
			expectError: false,
		},
		{
			name: "pipeline with missing timestamps",
			pipelineStr: `{
				"object_attributes": {
					"id": 123,
					"status": "success",
					"name": "Test Pipeline"
				},
				"project": {
					"path_with_namespace": "test/project"
				}
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pipelineEvent gitlab.PipelineEvent
			err := json.Unmarshal([]byte(tt.pipelineStr), &pipelineEvent)
			require.NoError(t, err, "failed to unmarshal test pipeline")

			pipeline := &glPipeline{&pipelineEvent}
			span := ptrace.NewSpan()

			err = pipeline.setSpanData(span)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify the span name
			if pipelineEvent.ObjectAttributes.Name != "" {
				require.Equal(t, pipelineEvent.ObjectAttributes.Name, span.Name())
			} else {
				require.Equal(t, pipelineEvent.Commit.Title, span.Name())
			}

			// Verify timestamps
			if pipelineEvent.ObjectAttributes.CreatedAt != "" && pipelineEvent.ObjectAttributes.FinishedAt != "" {
				startTime, _ := parseGitlabTime(pipelineEvent.ObjectAttributes.CreatedAt)
				endTime, _ := parseGitlabTime(pipelineEvent.ObjectAttributes.FinishedAt)

				require.Equal(t, pcommon.NewTimestampFromTime(startTime), span.StartTimestamp())
				require.Equal(t, pcommon.NewTimestampFromTime(endTime), span.EndTimestamp())
			}
		})
	}
}

func TestGlPipelineStageSetSpanData(t *testing.T) {
	tests := []struct {
		name        string
		stage       *glPipelineStage
		expectError bool
	}{
		{
			name: "valid stage with timestamps",
			stage: &glPipelineStage{
				PipelineID:         123,
				Name:               "test-stage",
				Status:             "success",
				PipelineFinishedAt: "2022-01-01 13:00:00 UTC",
				StartedAt:          "2022-01-01 12:00:00 UTC",
				FinishedAt:         "2022-01-01 12:30:00 UTC",
			},
			expectError: false,
		},
		{
			name: "stage with missing timestamps",
			stage: &glPipelineStage{
				PipelineID:         123,
				Name:               "test-stage",
				Status:             "success",
				PipelineFinishedAt: "2022-01-01 13:00:00 UTC",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			err := tt.stage.setSpanData(span)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify name
			require.Equal(t, tt.stage.Name, span.Name())

			// Verify timestamps
			if tt.stage.StartedAt != "" && tt.stage.FinishedAt != "" {
				startTime, _ := parseGitlabTime(tt.stage.StartedAt)
				endTime, _ := parseGitlabTime(tt.stage.FinishedAt)

				require.Equal(t, pcommon.NewTimestampFromTime(startTime), span.StartTimestamp())
				require.Equal(t, pcommon.NewTimestampFromTime(endTime), span.EndTimestamp())
			}
		})
	}
}

func TestGlPipelineJobSetSpanData(t *testing.T) {
	tests := []struct {
		name        string
		job         *glPipelineJob
		expectError bool
	}{
		{
			name: "valid job with timestamps",
			job: &glPipelineJob{
				event: &glJobEvent{
					ID:         123,
					Stage:      "test-stage",
					Name:       "test-job",
					Status:     "success",
					CreatedAt:  "2022-01-01 12:00:00 UTC",
					StartedAt:  "2022-01-01 12:01:00 UTC",
					FinishedAt: "2022-01-01 12:10:00 UTC",
					Runner: gitlab.PipelineEventBuildRunner{
						ID:          1,
						Description: "test-runner",
					},
				},
				jobURL: "https://example.com/-/jobs/123",
			},
			expectError: false,
		},
		{
			name: "job with missing timestamp",
			job: &glPipelineJob{
				event: &glJobEvent{
					ID:        123,
					Stage:     "test-stage",
					Name:      "test-job",
					Status:    "success",
					CreatedAt: "2022-01-01 12:00:00 UTC",
					Runner: gitlab.PipelineEventBuildRunner{
						ID:          1,
						Description: "test-runner",
					},
				},
				jobURL: "https://example.com/-/jobs/123",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			err := tt.job.setSpanData(span)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify name
			require.Equal(t, tt.job.event.Name, span.Name())

			// Verify timestamps
			if tt.job.event.StartedAt != "" && tt.job.event.FinishedAt != "" {
				startTime, _ := parseGitlabTime(tt.job.event.StartedAt)
				endTime, _ := parseGitlabTime(tt.job.event.FinishedAt)

				require.Equal(t, pcommon.NewTimestampFromTime(startTime), span.StartTimestamp())
				require.Equal(t, pcommon.NewTimestampFromTime(endTime), span.EndTimestamp())
			}
		})
	}
}
