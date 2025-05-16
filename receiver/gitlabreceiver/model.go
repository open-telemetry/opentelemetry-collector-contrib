// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	errSetPipelineTimestamps = errors.New("failed to set pipeline span timestamps")
	errSetStageTimestamps    = errors.New("failed to set stage span timestamps")
	errSetJobTimestamps      = errors.New("failed to set job span timestamps")
	errSetSpanAttributes     = errors.New("failed to set span attributes")
)

// glPipeline is a wrapper that implements the GitlabEvent interface
type glPipeline struct {
	*gitlab.PipelineEvent
}

func (p *glPipeline) setSpanData(span ptrace.Span) error {
	var pipelineName string
	if p.ObjectAttributes.Name != "" {
		pipelineName = p.ObjectAttributes.Name
	} else {
		pipelineName = p.Commit.Title
	}
	span.SetName(pipelineName)

	err := p.setTimeStamps(span, p.ObjectAttributes.CreatedAt, p.ObjectAttributes.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetPipelineTimestamps, err)
	}

	err = p.setAttributes(span)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetSpanAttributes, err)
	}

	return nil
}

// the glPipeline doesn't have a parent span ID, bc it's the root span
func (p *glPipeline) setSpanIDs(span ptrace.Span, spanID pcommon.SpanID) error {
	span.SetSpanID(spanID)
	return nil
}

func (p *glPipeline) setTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (p *glPipeline) setAttributes(_ ptrace.Span) error {
	// ToDo in next PR: set semconv attributes
	return nil
}

// glPipelineStage represents a stage in a pipeline event
type glPipelineStage struct {
	PipelineID         int
	Name               string
	Status             string
	PipelineFinishedAt string
	StartedAt          string
	FinishedAt         string
}

func (s *glPipelineStage) setSpanData(span ptrace.Span) error {
	span.SetName(s.Name)

	err := s.setTimeStamps(span, s.StartedAt, s.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetStageTimestamps, err)
	}

	err = s.setAttributes(span)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetSpanAttributes, err)
	}

	return nil
}

func (s *glPipelineStage) setSpanIDs(span ptrace.Span, parentSpanID pcommon.SpanID) error {
	span.SetParentSpanID(parentSpanID)

	stageSpanID, err := newStageSpanID(s.PipelineID, s.Name, s.StartedAt)
	if err != nil {
		return err
	}
	span.SetSpanID(stageSpanID)

	return nil
}

func (s *glPipelineStage) setTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (s *glPipelineStage) setAttributes(_ ptrace.Span) error {
	// ToDo in next PR: set semconv attributes
	return nil
}

// glPipelineJob represents a job in a pipeline event.
// This is a copy of the "PipelineEvent.Builds" struct in the Gitlab API client - it's not exported as type, so we need to use this struct to represent it
type glPipelineJob struct {
	ID             int               `json:"id"`
	Stage          string            `json:"stage"`
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	CreatedAt      string            `json:"created_at"`
	StartedAt      string            `json:"started_at"`
	FinishedAt     string            `json:"finished_at"`
	Duration       float64           `json:"duration"`
	QueuedDuration float64           `json:"queued_duration"`
	FailureReason  string            `json:"failure_reason"`
	When           string            `json:"when"`
	Manual         bool              `json:"manual"`
	AllowFailure   bool              `json:"allow_failure"`
	User           *gitlab.EventUser `json:"user"`
	Runner         struct {
		ID          int      `json:"id"`
		Description string   `json:"description"`
		Active      bool     `json:"active"`
		IsShared    bool     `json:"is_shared"`
		RunnerType  string   `json:"runner_type"`
		Tags        []string `json:"tags"`
	} `json:"runner"`
	ArtifactsFile struct {
		Filename string `json:"filename"`
		Size     int    `json:"size"`
	} `json:"artifacts_file"`
	Environment struct {
		Name           string `json:"name"`
		Action         string `json:"action"`
		DeploymentTier string `json:"deployment_tier"`
	} `json:"environment"`
}

func (j *glPipelineJob) setSpanData(span ptrace.Span) error {
	span.SetName(j.Name)

	err := j.setTimeStamps(span, j.StartedAt, j.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetJobTimestamps, err)
	}

	err = j.setAttributes(span)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetSpanAttributes, err)
	}

	return nil
}

func (j *glPipelineJob) setSpanIDs(span ptrace.Span, parentSpanID pcommon.SpanID) error {
	span.SetParentSpanID(parentSpanID)

	spanID, err := newJobSpanID(j.ID, j.StartedAt)
	if err != nil {
		return err
	}
	span.SetSpanID(spanID)
	return nil
}

func (j *glPipelineJob) setTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (j *glPipelineJob) setAttributes(_ ptrace.Span) error {
	// ToDo in next PR: set semconv attributes
	return nil
}
