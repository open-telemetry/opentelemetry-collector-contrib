// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
	"fmt"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

var (
	errSetPipelineTimestamps = errors.New("failed to set pipeline span timestamps")
	errSetStageTimestamps    = errors.New("failed to set stage span timestamps")
	errSetJobTimestamps      = errors.New("failed to set job span timestamps")
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
	span.SetKind(ptrace.SpanKindServer)

	err := p.setTimeStamps(span, p.ObjectAttributes.CreatedAt, p.ObjectAttributes.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetPipelineTimestamps, err)
	}

	p.setAttributes(span.Attributes())

	setSpanStatus(span, p.ObjectAttributes.Status)

	return nil
}

// the glPipeline doesn't have a parent span ID, bc it's the root span
func (*glPipeline) setSpanIDs(span ptrace.Span, spanID pcommon.SpanID) error {
	span.SetSpanID(spanID)
	return nil
}

func (*glPipeline) setTimeStamps(span ptrace.Span, startTime, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (p *glPipeline) setAttributes(attrs pcommon.Map) {
	// CICD attributes

	// ObjectAttributes.Name is the pipeline name, but it was added later in GitLab starting with version 16.1 (https://gitlab.com/gitlab-org/gitlab/-/merge_requests/123639)
	// The name isn't always present in the GitLab webhook events, therefore we need to check if it's empty
	if p.ObjectAttributes.Name != "" {
		attrs.PutStr(string(semconv.CICDPipelineNameKey), p.ObjectAttributes.Name)
	}

	attrs.PutStr(string(semconv.CICDPipelineResultKey), p.ObjectAttributes.Status) //ToDo: Check against well known values and compare with run.state
	attrs.PutStr(string(semconv.CICDPipelineRunIDKey), strconv.Itoa(p.ObjectAttributes.ID))
	attrs.PutStr(string(semconv.CICDPipelineRunURLFullKey), p.ObjectAttributes.URL)

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
	span.SetKind(ptrace.SpanKindServer)

	err := s.setTimeStamps(span, s.StartedAt, s.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetStageTimestamps, err)
	}

	setSpanStatus(span, s.Status)

	s.setAttributes(span.Attributes())

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

func (*glPipelineStage) setTimeStamps(span ptrace.Span, startTime, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

const (
	CICDPipelineTaskTypeStatusKey = "cicd.pipeline.task.type.status" //ToDo: check if we should add to semconv
)

func (s *glPipelineStage) setAttributes(attrs pcommon.Map) {
	attrs.PutStr(string(semconv.CICDPipelineTaskTypeKey), s.Name)
	attrs.PutStr(CICDPipelineTaskTypeStatusKey, s.Status)
}

// glJobEvent represents the anonymous struct type from PipelineEvent.Builds - it's not exported as type by the Gitlab API client, so we need to use this struct to represent it
type glJobEvent struct {
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

// glPipelineJob wraps a pointer to the GitLab job data with additional fields like jobUrl (required for span attributes)
type glPipelineJob struct {
	event  *glJobEvent
	jobURL string
}

func (j *glPipelineJob) setSpanData(span ptrace.Span) error {
	span.SetName(j.event.Name)
	span.SetKind(ptrace.SpanKindServer)

	err := j.setTimeStamps(span, j.event.StartedAt, j.event.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetJobTimestamps, err)
	}

	setSpanStatus(span, j.event.Status)

	j.setAttributes(span.Attributes())

	return nil
}

func (j *glPipelineJob) setSpanIDs(span ptrace.Span, parentSpanID pcommon.SpanID) error {
	span.SetParentSpanID(parentSpanID)

	spanID, err := newJobSpanID(j.event.ID, j.event.StartedAt)
	if err != nil {
		return err
	}
	span.SetSpanID(spanID)
	return nil
}

func (*glPipelineJob) setTimeStamps(span ptrace.Span, startTime, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (j *glPipelineJob) setAttributes(attrs pcommon.Map) {
	attrs.PutStr(string(semconv.CICDPipelineTaskNameKey), j.event.Name)
	attrs.PutStr(string(semconv.CICDPipelineTaskRunIDKey), strconv.Itoa(j.event.ID))
	attrs.PutStr(string(semconv.CICDPipelineTaskRunResultKey), j.event.Status)
	attrs.PutStr(string(semconv.CICDPipelineTaskRunURLFullKey), j.jobURL)

	// Runner attributes
	attrs.PutStr(string(semconv.CICDWorkerIDKey), strconv.Itoa(j.event.Runner.ID))
	attrs.PutStr(string(semconv.CICDWorkerNameKey), j.event.Runner.Description)

}
