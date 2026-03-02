// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

var (
	errTraceIDGeneration        = errors.New("failed to generate trace ID")
	errPipelineSpanIDGeneration = errors.New("failed to generate pipeline span ID")
	errPipelineSpanProcessing   = errors.New("failed to process pipeline span")
	errStageSpanProcessing      = errors.New("failed to process stage span")
	errJobSpanProcessing        = errors.New("failed to process job span")
)

const (
	gitlabEventTimeFormat = "2006-01-02 15:04:05 UTC"
)

// GitlabEvent abstracts span setup for different GitLab event types (pipeline, stage, job)
// It enables unified span creation logic while allowing type-specific customization
// It must be implemented by all GitLab event types (pipeline, stage, job) to create spans
type GitlabEvent interface {
	setAttributes(pcommon.Map)
	setSpanIDs(ptrace.Span, pcommon.SpanID) error
	setTimeStamps(ptrace.Span, string, string) error
	setSpanData(ptrace.Span) error
}

func (gtr *gitlabTracesReceiver) handlePipeline(e *gitlab.PipelineEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()

	gtr.setResourceAttributes(r.Resource().Attributes(), e)

	traceID, err := newTraceID(e.ObjectAttributes.ID, e.ObjectAttributes.FinishedAt)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("%w: %w", errTraceIDGeneration, err)
	}

	pipeline := &glPipeline{e}
	pipelineSpanID, err := newPipelineSpanID(pipeline.ObjectAttributes.ID, pipeline.ObjectAttributes.FinishedAt)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("%w: %w", errPipelineSpanIDGeneration, err)
	}

	if err = gtr.processPipelineSpan(r, pipeline, traceID, pipelineSpanID); err != nil {
		return ptrace.Traces{}, fmt.Errorf("%w: %w", errPipelineSpanProcessing, err)
	}

	stages, err := gtr.processStageSpans(r, pipeline, traceID, pipelineSpanID)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("%w: %w", errStageSpanProcessing, err)
	}

	if err := gtr.processJobSpans(r, pipeline, traceID, stages); err != nil {
		return ptrace.Traces{}, fmt.Errorf("%w: %w", errJobSpanProcessing, err)
	}

	return t, nil
}

func (gtr *gitlabTracesReceiver) processPipelineSpan(r ptrace.ResourceSpans, pipeline *glPipeline, traceID pcommon.TraceID, pipelineSpanID pcommon.SpanID) error {
	err := gtr.createSpan(r, pipeline, traceID, pipelineSpanID)
	if err != nil {
		return err
	}

	return nil
}

func (gtr *gitlabTracesReceiver) processStageSpans(r ptrace.ResourceSpans, pipeline *glPipeline, traceID pcommon.TraceID, parentSpanID pcommon.SpanID) (map[string]*glPipelineStage, error) {
	stages, err := gtr.newStages(pipeline)
	if err != nil {
		return nil, err
	}

	for _, stage := range stages {
		err = gtr.createSpan(r, stage, traceID, parentSpanID)
		if err != nil {
			return nil, err
		}
	}
	return stages, nil
}

func (gtr *gitlabTracesReceiver) processJobSpans(r ptrace.ResourceSpans, p *glPipeline, traceID pcommon.TraceID, stages map[string]*glPipelineStage) error {
	baseJobURL := p.Project.WebURL + "/-/jobs/"

	for i := range p.Builds {
		glJob := glPipelineJob{
			event:  (*glJobEvent)(&p.Builds[i]),
			jobURL: baseJobURL + strconv.FormatInt(p.Builds[i].ID, 10),
		}

		if glJob.event.FinishedAt != "" {
			parentSpanID, err := newStageSpanID(p.ObjectAttributes.ID, glJob.event.Stage, stages[glJob.event.Stage].StartedAt)
			if err != nil {
				return err
			}

			err = gtr.createSpan(r, &glJob, traceID, parentSpanID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (*gitlabTracesReceiver) createSpan(resourceSpans ptrace.ResourceSpans, e GitlabEvent, traceID pcommon.TraceID, spanID pcommon.SpanID) error {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	span.SetTraceID(traceID)

	err := e.setSpanIDs(span, spanID)
	if err != nil {
		return err
	}

	err = e.setSpanData(span)
	if err != nil {
		return err
	}

	return nil
}

// newTraceID creates a deterministic Trace ID based on the provided pipelineID and pipeline finishedAt time.
// It's not possible to create the traceID during a pipeline execution. Details can be found here: https://github.com/open-telemetry/semantic-conventions/issues/1749#issuecomment-2772544215
func newTraceID(pipelineID int64, finishedAt string) (pcommon.TraceID, error) {
	_, err := parseGitlabTime(finishedAt)
	if err != nil {
		return pcommon.TraceID{}, fmt.Errorf("invalid finishedAt timestamp: %w", err)
	}

	input := fmt.Sprintf("%dt%s", pipelineID, finishedAt)
	hash := sha256.Sum256([]byte(input))
	idHex := hex.EncodeToString(hash[:])

	var id pcommon.TraceID
	_, err = hex.Decode(id[:], []byte(idHex[:32]))
	if err != nil {
		return pcommon.TraceID{}, err
	}

	return id, nil
}

// newPipelineSpanID creates a deterministic Parent Span ID based on the provided pipelineID and pipeline finishedAt time.
// It's not possible to create the pipelineSpanID during a pipeline execution. Details can be found here: https://github.com/open-telemetry/semantic-conventions/issues/1749#issuecomment-2772544215
func newPipelineSpanID(pipelineID int64, finishedAt string) (pcommon.SpanID, error) {
	_, err := parseGitlabTime(finishedAt)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("invalid finishedAt timestamp: %w", err)
	}

	spanID, err := newSpanID(fmt.Sprintf("%d%s", pipelineID, finishedAt))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// newStageSpanID creates a deterministic Stage Span ID based on the provided pipelineID, stageName, and stage startedAt time.
// It's not possible to create the stageSpanID during a pipeline execution. Details can be found here: https://github.com/open-telemetry/semantic-conventions/issues/1749#issuecomment-2772544215
func newStageSpanID(pipelineID int64, stageName, startedAt string) (pcommon.SpanID, error) {
	if stageName == "" {
		return pcommon.SpanID{}, errors.New("stageName is empty")
	}

	_, err := parseGitlabTime(startedAt)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("invalid startedAt timestamp: %w", err)
	}

	spanID, err := newSpanID(fmt.Sprintf("%d%s%s", pipelineID, stageName, startedAt))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// newJobSpanID creates a deterministic Job Span ID based on the unique jobID
func newJobSpanID(jobID int64, startedAt string) (pcommon.SpanID, error) {
	_, err := parseGitlabTime(startedAt)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("invalid startedAt timestamp: %w", err)
	}

	spanID, err := newSpanID(fmt.Sprintf("%d%s", jobID, startedAt))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// newSpanID is a helper function to create a Span ID
func newSpanID(input string) (pcommon.SpanID, error) {
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// newStages extracts stage information from pipeline jobs.
// Since GitLab doesn't provide webhook events for stages within a pipeline, we need to create a new stage for each job.
func (gtr *gitlabTracesReceiver) newStages(pipeline *glPipeline) (map[string]*glPipelineStage, error) {
	stages := make(map[string]*glPipelineStage)

	for i := range pipeline.Builds {
		job := pipeline.Builds[i]
		stage, exists := stages[job.Stage]
		if !exists {
			stage = &glPipelineStage{
				PipelineID:         pipeline.ObjectAttributes.ID,
				Name:               job.Stage,
				Status:             job.Status,
				PipelineFinishedAt: pipeline.ObjectAttributes.FinishedAt,
			}
			stages[job.Stage] = stage
		}
		if err := gtr.setStageTime(stage, job.StartedAt, job.FinishedAt); err != nil {
			return nil, fmt.Errorf("updating stage timing for %s: %w", job.Stage, err)
		}
	}

	return stages, nil
}

// setStageTime determines stage start/finish times by finding the earliest start and latest finish time
func (*gitlabTracesReceiver) setStageTime(stage *glPipelineStage, jobStartedAt, jobFinishedAt string) error {
	// Handle start time
	if stage.StartedAt == "" {
		stage.StartedAt = jobStartedAt
	} else if jobStartedAt != "" {
		jobStartTime, err := parseGitlabTime(jobStartedAt)
		if err != nil {
			return fmt.Errorf("parsing job start time: %w", err)
		}

		stageStartTime, err := parseGitlabTime(stage.StartedAt)
		if err != nil {
			return fmt.Errorf("parsing stage start time: %w", err)
		}

		if jobStartTime.Before(stageStartTime) {
			stage.StartedAt = jobStartedAt
		}
	}

	// Handle finish time
	if stage.FinishedAt == "" {
		stage.FinishedAt = jobFinishedAt
	} else if jobFinishedAt != "" {
		jobFinishTime, err := parseGitlabTime(jobFinishedAt)
		if err != nil {
			return fmt.Errorf("parsing job finish time: %w", err)
		}

		stageFinishTime, err := parseGitlabTime(stage.FinishedAt)
		if err != nil {
			return fmt.Errorf("parsing stage finish time: %w", err)
		}

		if jobFinishTime.After(stageFinishTime) {
			stage.FinishedAt = jobFinishedAt
		}
	}

	return nil
}

func setSpanTimeStamps(span ptrace.Span, startTime, endTime string) error {
	parsedStartTime, err := parseGitlabTime(startTime)
	if err != nil {
		return err
	}
	parsedEndTime, err := parseGitlabTime(endTime)
	if err != nil {
		return err
	}

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(parsedStartTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(parsedEndTime))
	return nil
}

// parseGitlabTime parses the time string from the gitlab event, it differs between the test pipeline event and the actual webhook event,
// because the test pipeline event has a different time format than the actual webhook event.
// Test pipeline event time format: 2025-04-01T18:31:49.624Z
// Actual webhook event time format: 2025-04-01 18:31:49 UTC
func parseGitlabTime(t string) (time.Time, error) {
	if t == "" || t == "null" {
		return time.Time{}, errors.New("time is empty")
	}

	// Time format of actual webhook events
	pt, err := time.Parse(gitlabEventTimeFormat, t)
	if err == nil {
		return pt, nil
	}

	// Time format of test webhook events
	pt, err = time.Parse(time.RFC3339, t)
	if err == nil {
		return pt, nil
	}

	return time.Time{}, err
}

// map GitLab status to OTel span status and set optional message
// Relevant GitLab docs:
// - Job: https://docs.gitlab.com/ci/jobs/#available-job-statuses
// - Pipeline: https://docs.gitlab.com/api/pipelines/#list-project-pipelines (see status field)
func setSpanStatus(span ptrace.Span, status string) {
	switch strings.ToLower(status) {
	case pipelineStatusSuccess:
		span.Status().SetCode(ptrace.StatusCodeOk)
	case pipelineStatusFailed, pipelineStatusCanceled:
		span.Status().SetCode(ptrace.StatusCodeError)
	case pipelineStatusSkipped:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}
}

func (gtr *gitlabTracesReceiver) setResourceAttributes(attrs pcommon.Map, e *gitlab.PipelineEvent) {
	// Service
	attrs.PutStr(string(conventions.ServiceNameKey), e.Project.PathWithNamespace)

	// CICD
	attrs.PutStr(string(conventions.CICDPipelineNameKey), e.ObjectAttributes.Name)
	attrs.PutStr(string(conventions.CICDPipelineResultKey), e.ObjectAttributes.Status)
	attrs.PutInt(string(conventions.CICDPipelineRunIDKey), e.ObjectAttributes.ID)
	attrs.PutStr(string(conventions.CICDPipelineRunURLFullKey), e.ObjectAttributes.URL)

	// Resource attributes for workers are not applicable for GitLab, because GitLab provides worker information on job level
	// One pipeline can have multiple jobs, and each job can have a different worker
	// Therefore we set the worker attributes on job level

	// VCS
	attrs.PutStr(string(conventions.VCSProviderNameGitlab.Key), conventions.VCSProviderNameGitlab.Value.AsString())

	attrs.PutStr(string(conventions.VCSRepositoryNameKey), e.Project.Name)
	attrs.PutStr(string(conventions.VCSRepositoryURLFullKey), e.Project.WebURL)

	attrs.PutStr(string(conventions.VCSRefHeadNameKey), e.ObjectAttributes.Ref)
	refType := conventions.VCSRefTypeBranch.Value.AsString()
	if e.ObjectAttributes.Tag {
		refType = conventions.VCSRefTypeTag.Value.AsString()
	}
	attrs.PutStr(string(conventions.VCSRefHeadTypeKey), refType)
	attrs.PutStr(string(conventions.VCSRefHeadRevisionKey), e.ObjectAttributes.SHA)

	// Merge Request attributes (only for MR-triggered pipelines)
	if e.MergeRequest.ID != 0 {
		attrs.PutStr(string(conventions.VCSChangeIDKey), strconv.FormatInt(e.MergeRequest.ID, 10))
		attrs.PutStr(string(conventions.VCSChangeStateKey), e.MergeRequest.State)
		attrs.PutStr(string(conventions.VCSChangeTitleKey), e.MergeRequest.Title)
		attrs.PutStr(string(conventions.VCSRefBaseNameKey), e.MergeRequest.TargetBranch)
		attrs.PutStr(string(conventions.VCSRefBaseTypeKey), conventions.VCSRefTypeBranch.Value.AsString())
	}

	// ---------- The following attributes are not part of semconv yet ----------

	// VCS
	// We need to check if the commit timestamp is not nil, otherwise we might have a nil pointer dereference when calling Format()
	if e.Commit.Timestamp != nil {
		attrs.PutStr(AttributeVCSRefHeadRevisionTimestamp, e.Commit.Timestamp.Format(gitlabEventTimeFormat))
	}

	attrs.PutStr(AttributeVCSRepositoryVisibility, string(e.Project.Visibility))
	attrs.PutStr(AttributeGitLabProjectNamespace, e.Project.Namespace)
	attrs.PutStr(AttributeVCSRepositoryRefDefault, e.Project.DefaultBranch)

	// User details are only included if explicitly enabled in configuration
	if gtr.cfg.WebHook.IncludeUserAttributes {
		attrs.PutStr(AttributeVCSRefHeadRevisionAuthorName, e.Commit.Author.Name)
		attrs.PutStr(AttributeVCSRefHeadRevisionAuthorEmail, e.Commit.Author.Email)
		attrs.PutStr(AttributeVCSRefHeadRevisionMessage, e.Commit.Message)

		if e.User != nil {
			attrs.PutInt(AttributeCICDPipelineRunActorID, e.User.ID)
			attrs.PutStr(AttributeGitLabPipelineRunActorUsername, e.User.Username)
			attrs.PutStr(AttributeCICDPipelineRunActorName, e.User.Name)
		}
	}
}
