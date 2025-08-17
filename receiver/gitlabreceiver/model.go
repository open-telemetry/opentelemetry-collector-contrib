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
	errSetSpanAttributes     = errors.New("failed to set span attributes")
)

// glPipeline is a wrapper that implements the GitlabEvent interface
type glPipeline struct {
	*gitlab.PipelineEvent
}

func (p *glPipeline) setSpanData(span ptrace.Span, attrs pcommon.Map) error {
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

	setSpanStatusFromGitLab(span, p.ObjectAttributes.Status)

	err = p.setAttributes(attrs)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetSpanAttributes, err)
	}

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

func (p *glPipeline) setAttributes(attrs pcommon.Map) error {
	p.setVCSAttributes(attrs)
	p.setCICDAttributes(attrs)
	return nil
}

func (p *glPipeline) setVCSAttributes(attrs pcommon.Map) pcommon.Map {
	// Provider (GitLab)
	attrs.PutStr(string(semconv.VCSProviderNameGitlab.Key), semconv.VCSProviderNameGitlab.Value.AsString())

	// Repository
	attrs.PutStr(string(semconv.VCSRepositoryNameKey), p.Project.Name)
	attrs.PutStr(string(semconv.VCSRepositoryURLFullKey), p.Project.WebURL)

	// Merge Request (only applies to MR pipelines)
	if p.MergeRequest.ID != 0 {
		attrs.PutStr(string(semconv.VCSChangeIDKey), strconv.Itoa(p.MergeRequest.ID))
		attrs.PutStr(string(semconv.VCSChangeStateKey), p.MergeRequest.State)
		attrs.PutStr(string(semconv.VCSChangeTitleKey), p.MergeRequest.Title)
		attrs.PutStr(string(semconv.VCSRefBaseNameKey), p.MergeRequest.TargetBranch)
		attrs.PutStr(string(semconv.VCSRefBaseTypeKey), semconv.VCSRefTypeBranch.Value.AsString())

		// Head Branch
		attrs.PutStr(string(semconv.VCSRefHeadNameKey), p.ObjectAttributes.Ref)
		if !p.ObjectAttributes.Tag {
			attrs.PutStr(string(semconv.VCSRefHeadTypeKey), semconv.VCSRefTypeBranch.Value.AsString())
		} else {
			attrs.PutStr(string(semconv.VCSRefHeadTypeKey), semconv.VCSRefTypeTag.Value.AsString())
		}
		attrs.PutStr(string(semconv.VCSRefHeadRevisionKey), p.ObjectAttributes.SHA)
	}

	return attrs
}

func (p *glPipeline) setCICDAttributes(attrs pcommon.Map) pcommon.Map {
	// ObjectAttributes.Name is the pipeline name, but it was added later in GitLab starting with version 16.1 (https://gitlab.com/gitlab-org/gitlab/-/merge_requests/123639)
	// The name isn't always present in the GitLab webhook events, therefore we need to check if it's empty
	if p.ObjectAttributes.Name != "" {
		attrs.PutStr(string(semconv.CICDPipelineNameKey), p.ObjectAttributes.Name)
	}

	attrs.PutStr(string(semconv.CICDPipelineResultKey), p.ObjectAttributes.Status) //ToDo: Check against well known values and compare with run.state
	attrs.PutStr(string(semconv.CICDPipelineRunIDKey), strconv.Itoa(p.ObjectAttributes.ID))
	attrs.PutStr(string(semconv.CICDPipelineRunURLFullKey), p.ObjectAttributes.URL)

	return attrs
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

func (s *glPipelineStage) setSpanData(span ptrace.Span, attrs pcommon.Map) error {
	span.SetName(s.Name)
	span.SetKind(ptrace.SpanKindServer)

	err := s.setTimeStamps(span, s.StartedAt, s.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetStageTimestamps, err)
	}

	setSpanStatusFromGitLab(span, s.Status)

	err = s.setAttributes(attrs)
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

func (*glPipelineStage) setTimeStamps(span ptrace.Span, startTime, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

const (
	CICDPipelineTaskTypeStatusKey = "cicd.pipeline.task.type.status" //ToDo: check if we should add to semconv
)

func (s *glPipelineStage) setAttributes(attrs pcommon.Map) error {
	attrs.PutStr(string(semconv.CICDPipelineTaskTypeKey), s.Name)
	attrs.PutStr(CICDPipelineTaskTypeStatusKey, s.Status)
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

func (j *glPipelineJob) setSpanData(span ptrace.Span, attrs pcommon.Map) error {
	span.SetName(j.Name)
	span.SetKind(ptrace.SpanKindServer)

	err := j.setTimeStamps(span, j.StartedAt, j.FinishedAt)
	if err != nil {
		return fmt.Errorf("%w: %w", errSetJobTimestamps, err)
	}

	setSpanStatusFromGitLab(span, j.Status)

	err = j.setAttributes(attrs)
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

func (*glPipelineJob) setTimeStamps(span ptrace.Span, startTime, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func (j *glPipelineJob) setAttributes(attrs pcommon.Map) error {
	attrs.PutStr(string(semconv.CICDPipelineTaskNameKey), j.Name)
	attrs.PutStr(string(semconv.CICDPipelineTaskRunIDKey), strconv.Itoa(j.ID))
	attrs.PutStr(string(semconv.CICDPipelineTaskRunResultKey), j.Status)
	// attrs.PutStr(string(semconv.CICDPipelineTaskRunURLFullKey), fmt.Sprintf("%s/jobs/%d", p.ObjectAttributes.URL, j.ID)	) - ToDo: We need to pass in pipeline attrs somehow

	// Runner Attributes
	attrs.PutStr(string(semconv.CICDWorkerIDKey), strconv.Itoa(j.Runner.ID))
	attrs.PutStr(string(semconv.CICDWorkerNameKey), j.Runner.Description)

	return nil
}
