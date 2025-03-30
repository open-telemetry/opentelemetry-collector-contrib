package gitlabreceiver

import (
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Wrapper is required to have a type that implements the GitlabEvent interface
type GitlabPipelineEvent struct {
	*gitlab.PipelineEvent
}

func (p *GitlabPipelineEvent) setAttributes(span ptrace.Span) error {
	var pipelineName string
	if p.ObjectAttributes.Name != "" {
		pipelineName = p.ObjectAttributes.Name
	} else {
		pipelineName = p.Commit.Title
	}
	span.SetName(pipelineName)

	//ToDo: Set semconv attributes
	err := p.setTimeStamps(span, p.ObjectAttributes.CreatedAt, p.ObjectAttributes.FinishedAt)
	if err != nil {
		return fmt.Errorf("failed to set pipeline span timestamps: %w", err)
	}

	return nil
}

func (p *GitlabPipelineEvent) setSpanID(span ptrace.Span, parentSpanID pcommon.SpanID) error {
	span.SetSpanID(parentSpanID)

	return nil
}

func (p *GitlabPipelineEvent) setTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

// Build represents a job in a pipeline event (the type is not exported from the Gitlab SDK)
type GitlabPipelineJobEvent struct {
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

func (j *GitlabPipelineJobEvent) setAttributes(span ptrace.Span) error {
	jobName := fmt.Sprintf(gitlabJobName, j.Name, j.Status, j.Stage)
	span.SetName(jobName)

	//ToDo: set semconv attributes
	err := j.setTimeStamps(span, j.CreatedAt, j.FinishedAt)
	if err != nil {
		return fmt.Errorf("failed to set job span timestamps: %w", err)
	}
	return nil
}

func (j *GitlabPipelineJobEvent) setSpanID(span ptrace.Span, parentSpanID pcommon.SpanID) error {
	span.SetParentSpanID(parentSpanID)

	spanID, err := newJobSpanID(j.ID)
	if err != nil {
		return fmt.Errorf("failed to generate job span ID: %w", err)
	}
	span.SetSpanID(spanID)
	return nil
}

func (j *GitlabPipelineJobEvent) setTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	return setSpanTimeStamps(span, startTime, endTime)
}

func setSpanTimeStamps(span ptrace.Span, startTime string, endTime string) error {
	createdAt, err := newTimestampFromGitlabTime(startTime)
	if err != nil {
		return fmt.Errorf("failed to parse CreatedAt timestamp: %w", err)
	}
	span.SetStartTimestamp(createdAt)

	finishedAt, err := newTimestampFromGitlabTime(endTime)
	if err != nil {
		return fmt.Errorf("failed to parse FinishedAt timestamp: %w", err)
	}
	span.SetEndTimestamp(finishedAt)

	return nil
}
