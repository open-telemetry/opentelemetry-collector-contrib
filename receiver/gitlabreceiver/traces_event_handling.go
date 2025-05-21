// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
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
	setAttributes(ptrace.Span) error
	setSpanIDs(ptrace.Span, pcommon.SpanID) error
	setTimeStamps(ptrace.Span, string, string) error
	setSpanData(ptrace.Span) error
}

func (gtr *gitlabTracesReceiver) handlePipeline(e *gitlab.PipelineEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()
	r.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), e.Project.PathWithNamespace)

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
	for _, job := range p.Builds {
		jobEvent := glPipelineJob(job)

		if job.FinishedAt != "" {
			parentSpanID, err := newStageSpanID(p.ObjectAttributes.ID, job.Stage, stages[job.Stage].StartedAt)
			if err != nil {
				return err
			}

			err = gtr.createSpan(r, &jobEvent, traceID, parentSpanID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (gtr *gitlabTracesReceiver) createSpan(resourceSpans ptrace.ResourceSpans, e GitlabEvent, traceID pcommon.TraceID, spanID pcommon.SpanID) error {
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
func newTraceID(pipelineID int, finishedAt string) (pcommon.TraceID, error) {
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
func newPipelineSpanID(pipelineID int, finishedAt string) (pcommon.SpanID, error) {
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
func newStageSpanID(pipelineID int, stageName string, startedAt string) (pcommon.SpanID, error) {
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
func newJobSpanID(jobID int, startedAt string) (pcommon.SpanID, error) {
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

	for _, job := range pipeline.Builds {
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
		if err := gtr.setStageTime(stage, job); err != nil {
			return nil, fmt.Errorf("updating stage timing for %s: %w", job.Stage, err)
		}
	}

	return stages, nil
}

// setStageTime determines stage start/finish times by finding the earliest start and latest finish time
func (gtr *gitlabTracesReceiver) setStageTime(stage *glPipelineStage, job glPipelineJob) error {
	// Handle start time
	if stage.StartedAt == "" {
		stage.StartedAt = job.StartedAt
	} else if job.StartedAt != "" {
		jobStartTime, err := parseGitlabTime(job.StartedAt)
		if err != nil {
			return fmt.Errorf("parsing job start time: %w", err)
		}

		stageStartTime, err := parseGitlabTime(stage.StartedAt)
		if err != nil {
			return fmt.Errorf("parsing stage start time: %w", err)
		}

		if jobStartTime.Before(stageStartTime) {
			stage.StartedAt = job.StartedAt
		}
	}

	// Handle finish time
	if stage.FinishedAt == "" {
		stage.FinishedAt = job.FinishedAt
	} else if job.FinishedAt != "" {
		jobFinishTime, err := parseGitlabTime(job.FinishedAt)
		if err != nil {
			return fmt.Errorf("parsing job finish time: %w", err)
		}

		stageFinishTime, err := parseGitlabTime(stage.FinishedAt)
		if err != nil {
			return fmt.Errorf("parsing stage finish time: %w", err)
		}

		if jobFinishTime.After(stageFinishTime) {
			stage.FinishedAt = job.FinishedAt
		}
	}

	return nil
}

func setSpanTimeStamps(span ptrace.Span, startTime string, endTime string) error {
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
