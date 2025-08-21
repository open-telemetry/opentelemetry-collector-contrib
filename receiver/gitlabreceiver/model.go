// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

const (
	// ---------- SEMCONV-ALIGNED (NOT IN SEMCONV YET) ----------
	// Follow semconv patterns, candidates for future inclusion, highly experimental and subject to change

	// Repository
	AttributeVCSRepositoryVisibility = "vcs.repository.visibility"
	AttributeVCSRepositoryNamespace  = "vcs.repository.namespace"
	AttributeVCSRepositoryRefDefault = "vcs.repository.ref.default"

	// Commit (might contain sensitive information)
	AttributeVCSRefHeadRevisionMessage     = "vcs.ref.head.revision.message"
	AttributeVCSRefHeadRevisionTimestamp   = "vcs.ref.head.revision.timestamp"
	AttributeVCSRefHeadRevisionAuthorName  = "vcs.ref.head.revision.author.name"
	AttributeVCSRefHeadRevisionAuthorEmail = "vcs.ref.head.revision.author.email"

	// Pipeline user (might contain sensitive information)
	AttributeCICDPipelineRunActorID       = "cicd.pipeline.run.actor.id"
	AttributeCICDPipelineRunActorUsername = "cicd.pipeline.run.actor.username"
	AttributeCICDPipelineRunActorName     = "cicd.pipeline.run.actor.name"

	// Task type
	AttributeCICDTaskTypeStatus = "cicd.pipeline.task.type.status"

	// Task run
	AttributeCICDTaskEnvironmentName = "cicd.pipeline.task.run.environment.name"

	// Runner/Worker
	AttributeCICDWorkerType   = "cicd.worker.type"
	AttributeCICDWorkerTags   = "cicd.worker.tags"
	AttributeCICDWorkerShared = "cicd.worker.shared"

	// The following attributes are EXCLUSIVE to GitLab but not listed under Vendor Extensions within Semantic Conventions yet
	// They are highly experimental and subject to change

	AttributeGitlabPipelineSource              = "gitlab.pipeline.source" // Source of the pipeline: https://docs.gitlab.com/ci/jobs/job_rules/#ci_pipeline_source-predefined-variable
	AttributeGitlabPipelineSourcePipelineID    = "gitlab.pipeline.source.pipeline.id"
	AttributeGitlabPipelineSourcePipelineJobID = "gitlab.pipeline.source.pipeline.job.id"

	AttributeGitlabPipelineSourcePipelineProjectID        = "gitlab.pipeline.source.pipeline.project.id"
	AttributeGitlabPipelineSourcePipelineProjectNamespace = "gitlab.pipeline.source.pipeline.project.namespace"
	AttributeGitlabPipelineSourcePipelineProjectURL       = "gitlab.pipeline.source.pipeline.project.url"

	// Job
	AttributeGitlabJobQueuedDuration = "gitlab.job.queued_duration"
	AttributeGitlabJobFailureReason  = "gitlab.job.failure_reason"
	AttributeGitlabJobAllowFailure   = "gitlab.job.allow_failure"

	// Environment
	AttributeGitlabEnvironmentAction         = "gitlab.environment.action"
	AttributeGitlabEnvironmentDeploymentTier = "gitlab.environment.deployment_tier"
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
	// ---------- The following attributes are not part of semconv yet ----------

	// Pipeline
	putStrIfNotEmpty(attrs, AttributeGitlabPipelineSource, p.ObjectAttributes.Source)

	// Parent Pipeline (only added if it's a multi-pipeline)
	if p.SourcePipeline.PipelineID > 0 {
		attrs.PutInt(AttributeGitlabPipelineSourcePipelineID, int64(p.SourcePipeline.PipelineID))
		putIntIfNotZero(attrs, AttributeGitlabPipelineSourcePipelineProjectID, int64(p.SourcePipeline.Project.ID))
		putIntIfNotZero(attrs, AttributeGitlabPipelineSourcePipelineJobID, int64(p.SourcePipeline.JobID))
		putStrIfNotEmpty(attrs, AttributeGitlabPipelineSourcePipelineProjectNamespace, p.SourcePipeline.Project.PathWithNamespace)
		putStrIfNotEmpty(attrs, AttributeGitlabPipelineSourcePipelineProjectURL, p.SourcePipeline.Project.WebURL)
	}
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

func (s *glPipelineStage) setAttributes(attrs pcommon.Map) {
	putStrIfNotEmpty(attrs, string(semconv.CICDPipelineTaskTypeKey), s.Name)

	// ---------- The following attribute is not part of semconv yet ----------
	putStrIfNotEmpty(attrs, AttributeCICDTaskTypeStatus, s.Status)
}

// glJobEvent represents the PipelineEvent.Builds struct from the pipeline webhook event - it's not exported as type by the Gitlab API client, so we need to use this struct to represent it
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

// glPipelineJob wraps the job event and adds additional fields like jobUrl (required for span attributes)
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
	// Job
	putStrIfNotEmpty(attrs, string(semconv.CICDPipelineTaskNameKey), j.event.Name)
	putIntIfNotZero(attrs, string(semconv.CICDPipelineTaskRunIDKey), int64(j.event.ID))
	putStrIfNotEmpty(attrs, string(semconv.CICDPipelineTaskRunResultKey), j.event.Status)
	putStrIfNotEmpty(attrs, string(semconv.CICDPipelineTaskRunURLFullKey), j.jobURL)

	// Worker/Runner
	putIntIfNotZero(attrs, string(semconv.CICDWorkerIDKey), int64(j.event.Runner.ID))
	putStrIfNotEmpty(attrs, string(semconv.CICDWorkerNameKey), j.event.Runner.Description)

	// ---------- The following attributes are not part of semconv yet ----------

	// Job
	putDoubleIfNotZero(attrs, AttributeGitlabJobQueuedDuration, j.event.QueuedDuration)
	putStrIfNotEmpty(attrs, AttributeGitlabJobFailureReason, j.event.FailureReason)
	attrs.PutBool(AttributeGitlabJobAllowFailure, j.event.AllowFailure)

	// Environment
	putStrIfNotEmpty(attrs, AttributeCICDTaskEnvironmentName, j.event.Environment.Name)
	putStrIfNotEmpty(attrs, AttributeGitlabEnvironmentDeploymentTier, j.event.Environment.DeploymentTier)
	putStrIfNotEmpty(attrs, AttributeGitlabEnvironmentAction, j.event.Environment.Action)

	// Worker/Runner
	if len(j.event.Runner.Tags) > 0 {
		labels := attrs.PutEmptySlice(AttributeCICDWorkerTags)
		labels.EnsureCapacity(len(j.event.Runner.Tags))
		for _, label := range j.event.Runner.Tags {
			l := strings.ToLower(label)
			labels.AppendEmpty().SetStr(l)
		}
	}

	putStrIfNotEmpty(attrs, AttributeCICDWorkerType, j.event.Runner.RunnerType)
	attrs.PutBool(AttributeCICDWorkerShared, j.event.Runner.IsShared)
}
