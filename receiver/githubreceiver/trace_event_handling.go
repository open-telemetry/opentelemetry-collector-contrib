// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v71/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

func (gtr *githubTracesReceiver) handleWorkflowRun(e *github.WorkflowRunEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()

	resource := r.Resource()

	err := gtr.getWorkflowRunAttrs(resource, e)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to get workflow run attributes: %w", err)
	}

	traceID, err := newTraceID(e.GetWorkflowRun().GetID(), e.GetWorkflowRun().GetRunAttempt())
	if err != nil {
		gtr.logger.Sugar().Error("failed to generate trace ID", zap.Error(err))
	}

	err = gtr.createRootSpan(r, e, traceID)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create root span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create root span")
	}
	return t, nil
}

// handleWorkflowJob handles the creation of spans for a GitHub Workflow Job
// events, including the underlying steps within each job. A `job` maps to the
// semantic conventions for a `cicd.pipeline.task`.
func (gtr *githubTracesReceiver) handleWorkflowJob(e *github.WorkflowJobEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()

	resource := r.Resource()

	err := gtr.getWorkflowJobAttrs(resource, e)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to get workflow run attributes: %w", err)
	}

	traceID, err := newTraceID(e.GetWorkflowJob().GetRunID(), int(e.GetWorkflowJob().GetRunAttempt()))
	if err != nil {
		gtr.logger.Sugar().Error("failed to generate trace ID", zap.Error(err))
	}

	parentID, err := gtr.createParentSpan(r, e, traceID)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create parent span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create parent span")
	}

	queueSpanID, err := gtr.createJobQueueSpan(r, e, traceID, parentID)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create job queue span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create job queue span")
	}

	err = gtr.createStepSpans(r, e, traceID, queueSpanID)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create step spans", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create step spans")
	}

	return t, nil
}

// newTraceID creates a deterministic Trace ID based on the provided inputs of
// runID and runAttempt. `t` is appended to the end of the input to
// differentiate between a deterministic traceID and the parentSpanID.
func newTraceID(runID int64, runAttempt int) (pcommon.TraceID, error) {
	input := fmt.Sprintf("%d%dt", runID, runAttempt)
	// TODO: Determine if this is the best hashing algorithm to use. This is
	// more likely to generate a unique hash compared to MD5 or SHA1. Could
	// alternatively use UUID library to generate a unique ID by also using a
	// hash.
	hash := sha256.Sum256([]byte(input))
	idHex := hex.EncodeToString(hash[:])

	var id pcommon.TraceID
	_, err := hex.Decode(id[:], []byte(idHex[:32]))
	if err != nil {
		return pcommon.TraceID{}, err
	}

	return id, nil
}

// newParentId creates a deterministic Parent Span ID based on the provided
// runID and runAttempt. `s` is appended to the end of the input to
// differentiate between a deterministic traceID and the parentSpanID.
func newParentSpanID(runID int64, runAttempt int) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%ds", runID, runAttempt)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// createRootSpan creates a root span based on the provided event, associated
// with the deterministic traceID.
func (gtr *githubTracesReceiver) createRootSpan(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowRunEvent,
	traceID pcommon.TraceID,
) error {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	rootSpanID, err := newParentSpanID(event.GetWorkflowRun().GetID(), event.GetWorkflowRun().GetRunAttempt())
	if err != nil {
		return fmt.Errorf("failed to generate root span ID: %w", err)
	}

	span.SetTraceID(traceID)
	span.SetSpanID(rootSpanID)
	span.SetName(event.GetWorkflowRun().GetName())
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowRun().GetRunStartedAt().Time))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowRun().GetUpdatedAt().Time))

	switch strings.ToLower(event.WorkflowRun.GetConclusion()) {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowRun().GetConclusion())

	// Attempt to link to previous trace ID if applicable
	if event.GetWorkflowRun().GetPreviousAttemptURL() != "" && event.GetWorkflowRun().GetRunAttempt() > 1 {
		gtr.logger.Debug("Linking to previous trace ID for WorkflowRunEvent")
		previousRunAttempt := event.GetWorkflowRun().GetRunAttempt() - 1
		previousTraceID, err := newTraceID(event.GetWorkflowRun().GetID(), previousRunAttempt)
		if err != nil {
			return fmt.Errorf("failed to generate previous traceID: %w", err)
		}

		link := span.Links().AppendEmpty()
		link.SetTraceID(previousTraceID)
		gtr.logger.Debug("successfully linked to previous trace ID", zap.String("previousTraceID", previousTraceID.String()))
	}

	return nil
}

// createParentSpan creates a parent span based on the provided event, associated
// with the deterministic traceID.
func (gtr *githubTracesReceiver) createParentSpan(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowJobEvent,
	traceID pcommon.TraceID,
) (pcommon.SpanID, error) {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	parentSpanID, err := newParentSpanID(event.GetWorkflowJob().GetRunID(), int(event.GetWorkflowJob().GetRunAttempt()))
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("failed to generate parent span ID: %w", err)
	}

	jobSpanID, err := newJobSpanID(event.GetWorkflowJob().GetRunID(), int(event.GetWorkflowJob().GetRunAttempt()), event.GetWorkflowJob().GetName())
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("failed to generate job span ID: %w", err)
	}

	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)
	span.SetSpanID(jobSpanID)
	span.SetName(event.GetWorkflowJob().GetName())
	span.SetKind(ptrace.SpanKindServer)

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowJob().GetCreatedAt().Time))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowJob().GetCompletedAt().Time))

	switch strings.ToLower(event.WorkflowJob.GetConclusion()) {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowJob().GetConclusion())

	return jobSpanID, nil
}

// newJobSpanId creates a deterministic Job Span ID based on the provided runID,
// runAttempt, and the name of the job.
func newJobSpanID(runID int64, runAttempt int, jobName string) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%d%s", runID, runAttempt, jobName)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// createStepSpans is a wrapper function to create spans for each step in the
// the workflow job by identifying duplicate names then creating a span for each
// step.
func (gtr *githubTracesReceiver) createStepSpans(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowJobEvent,
	traceID pcommon.TraceID,
	parentSpanID pcommon.SpanID,
) error {
	steps := event.GetWorkflowJob().Steps
	unique := newUniqueSteps(steps)
	var errors error
	for i, step := range steps {
		name := unique[i]
		err := gtr.createStepSpan(resourceSpans, traceID, parentSpanID, event, step, name)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
	}
	return errors
}

// newUniqueSteps creates a new slice of step names from the provided GitHub
// event steps. Each step name, if duplicated, is appended with `-n` where n is
// the numbered occurrence.
func newUniqueSteps(steps []*github.TaskStep) []string {
	if len(steps) == 0 {
		return nil
	}

	results := make([]string, len(steps))

	count := make(map[string]int, len(steps))
	for _, step := range steps {
		count[step.GetName()]++
	}

	occurrences := make(map[string]int, len(steps))
	for i, step := range steps {
		name := step.GetName()
		if count[name] == 1 {
			results[i] = name
			continue
		}

		occurrences[name]++
		if occurrences[name] == 1 {
			results[i] = name
		} else {
			results[i] = fmt.Sprintf("%s-%d", name, occurrences[name]-1)
		}
	}

	return results
}

// createStepSpan creates a span with a deterministic spandID for the provided
// step.
func (gtr *githubTracesReceiver) createStepSpan(
	resourceSpans ptrace.ResourceSpans,
	traceID pcommon.TraceID,
	parentSpanID pcommon.SpanID,
	event *github.WorkflowJobEvent,
	step *github.TaskStep,
	name string,
) error {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	span.SetName(name)
	span.SetKind(ptrace.SpanKindServer)
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	runID := event.GetWorkflowJob().GetRunID()
	runAttempt := int(event.GetWorkflowJob().GetRunAttempt())
	jobName := event.GetWorkflowJob().GetName()
	stepName := step.GetName()
	number := int(step.GetNumber())
	spanID, err := newStepSpanID(runID, runAttempt, jobName, stepName, number)
	if err != nil {
		return fmt.Errorf("failed to generate step span ID: %w", err)
	}

	span.SetSpanID(spanID)

	attrs := span.Attributes()
	attrs.PutStr(semconv.AttributeCicdPipelineTaskName, name)
	attrs.PutStr(AttributeCICDPipelineTaskRunStatus, step.GetStatus())
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(step.GetStartedAt().Time))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(step.GetCompletedAt().Time))

	switch strings.ToLower(step.GetConclusion()) {
	case "success":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusSuccess)
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusFailure)
		span.Status().SetCode(ptrace.StatusCodeError)
	case "skipped":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusFailure)
		span.Status().SetCode(ptrace.StatusCodeUnset)
	case "cancelled":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusCancellation)
		span.Status().SetCode(ptrace.StatusCodeUnset)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowJob().GetConclusion())

	return nil
}

// newStepSpanID creates a deterministic Step Span ID based on the provided
// inputs.
func newStepSpanID(runID int64, runAttempt int, jobName string, stepName string, number int) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%d%s%s%d", runID, runAttempt, jobName, stepName, number)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

// createJobQueueSpan creates a span for the job queue based on the provided
// event by using the delta between the job created and completed times.
func (gtr *githubTracesReceiver) createJobQueueSpan(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowJobEvent,
	traceID pcommon.TraceID,
	parentSpanID pcommon.SpanID,
) (pcommon.SpanID, error) {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	jobName := event.GetWorkflowJob().GetName()
	spanName := fmt.Sprintf("queue-%s", jobName)

	span.SetName(spanName)
	span.SetKind(ptrace.SpanKindServer)
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	runID := event.GetWorkflowJob().GetRunID()
	runAttempt := int(event.GetWorkflowJob().GetRunAttempt())
	spanID, err := newStepSpanID(runID, runAttempt, jobName, spanName, 1)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("failed to generate step span ID: %w", err)
	}

	span.SetSpanID(spanID)

	time := event.GetWorkflowJob().GetStartedAt().Sub(event.GetWorkflowJob().GetCreatedAt().Time)

	attrs := span.Attributes()
	attrs.PutDouble(AttributeCICDPipelineRunQueueDuration, float64(time.Nanoseconds()))

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowJob().GetCreatedAt().Time))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowJob().GetStartedAt().Time))

	return spanID, nil
}
