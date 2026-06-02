// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v86/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

func (gtr *githubTracesReceiver) handleWorkflowRun(e *github.WorkflowRunEvent, rawPayload []byte) (ptrace.Traces, error) {
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

	err = gtr.createRootSpan(r, e, traceID, rawPayload)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create root span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create root span")
	}
	return t, nil
}

// handleWorkflowJob handles the creation of spans for a GitHub Workflow Job
// events, including the underlying steps within each job. A `job` maps to the
// semantic conventions for a `cicd.pipeline.task`.
func (gtr *githubTracesReceiver) handleWorkflowJob(e *github.WorkflowJobEvent, rawPayload []byte) (ptrace.Traces, error) {
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

	parentID, err := gtr.createParentSpan(r, e, traceID, rawPayload)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create parent span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create parent span")
	}

	err = gtr.createJobQueueSpan(r, e, traceID, parentID)
	if err != nil {
		gtr.logger.Sugar().Error("failed to create job queue span", zap.Error(err))
		return ptrace.Traces{}, errors.New("failed to create job queue span")
	}

	err = gtr.createStepSpans(r, e, traceID, parentID)
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

// correctActionTimestamps ensures span timestamps are valid by checking that
// the end time is not before the start time. When GitHub reports timestamps
// in reverse order (which can occur with skipped jobs and steps), this function
// returns corrected timestamps where both start and end are set to the later
// timestamp, resulting in a zero-duration span.
//
// This prevents negative durations that would otherwise appear as excessively
// long spans in telemetry systems.
func correctActionTimestamps(start, end time.Time) (time.Time, time.Time) {
	if end.Before(start) {
		// Use the later timestamp (start) for both, creating zero-duration span
		return start, start
	}
	return start, end
}

// createRootSpan creates a root span based on the provided event, associated
// with the deterministic traceID.
func (gtr *githubTracesReceiver) createRootSpan(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowRunEvent,
	traceID pcommon.TraceID,
	rawPayload []byte,
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
	startTime, endTime := correctActionTimestamps(event.GetWorkflowRun().GetRunStartedAt().Time, event.GetWorkflowRun().GetUpdatedAt().Time)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	switch strings.ToLower(event.WorkflowRun.GetConclusion()) {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowRun().GetConclusion())

	// Attach raw event as span event if configured
	if gtr.cfg.WebHook.IncludeSpanEvents {
		spanEvent := span.Events().AppendEmpty()
		spanEvent.SetName("github.workflow_run.event")
		spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowRun().GetRunStartedAt().Time))
		spanEvent.Attributes().PutStr("event.payload", string(rawPayload))
	}

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
	rawPayload []byte,
) (pcommon.SpanID, error) {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	parentSpanID, err := newParentSpanID(event.GetWorkflowJob().GetRunID(), int(event.GetWorkflowJob().GetRunAttempt()))
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("failed to generate parent span ID: %w", err)
	}

	var jobSpanID pcommon.SpanID
	if metadata.ReceiverGithubreceiverUseCheckRunIDFeatureGate.IsEnabled() {
		checkRunID := event.GetWorkflowJob().GetID()
		if checkRunID == 0 {
			return pcommon.SpanID{}, fmt.Errorf(
				"failed to derive job parent span ID; workflow_job.id is missing for: repo=%q job=%q run_id=%d run_attempt=%d",
				event.GetRepo().GetFullName(),
				event.GetWorkflowJob().GetName(),
				event.GetWorkflowJob().GetRunID(),
				event.GetWorkflowJob().GetRunAttempt(),
			)
		}
		input := fmt.Sprintf("%d-j", checkRunID)
		jobSpanID, err = newSpanIDFromCheckRun(input)
	} else {
		jobSpanID, err = newJobSpanID(
			event.GetWorkflowJob().GetRunID(),
			int(event.GetWorkflowJob().GetRunAttempt()),
			event.GetWorkflowJob().GetName(),
		)
	}
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("failed to generate job span ID: %w", err)
	}

	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)
	span.SetSpanID(jobSpanID)
	span.SetName(event.GetWorkflowJob().GetName())
	span.SetKind(ptrace.SpanKindInternal)

	startTime, endTime := correctActionTimestamps(event.GetWorkflowJob().GetCreatedAt().Time, event.GetWorkflowJob().GetCompletedAt().Time)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	switch strings.ToLower(event.WorkflowJob.GetConclusion()) {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.GetWorkflowJob().GetConclusion())

	// Attach raw event as span event if configured
	if gtr.cfg.WebHook.IncludeSpanEvents {
		spanEvent := span.Events().AppendEmpty()
		spanEvent.SetName("github.workflow_job.event")
		spanEvent.SetTimestamp(pcommon.NewTimestampFromTime(event.GetWorkflowJob().GetCreatedAt().Time))
		spanEvent.Attributes().PutStr("event.payload", string(rawPayload))
	}

	return jobSpanID, nil
}

// newJobSpanID creates a deterministic Job Span ID based on the provided runID,
// runAttempt, and the name of the job.
//
// Deprecated: superseded by newSpanIDFromCheckRun, which is used when the
// receiver.githubreceiver.UseCheckRunID feature gate is enabled (the default as
// of v0.151.0). This function and its callers are scheduled for removal when
// the gate is promoted to Stable. See
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/44856.
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
	unique, hasDup := newUniqueSteps(steps)
	if hasDup && metadata.ReceiverGithubreceiverUseCheckRunIDFeatureGate.IsEnabled() {
		gtr.logger.Warn(
			"duplicate step names detected; steps sharing a name will have colliding span IDs. "+
				"Use unique step names within a job for reliable TRACEPARENT reproduction.",
			zap.String("workflow_job.name", event.GetWorkflowJob().GetName()),
			zap.Int64("workflow_job.run.id", event.GetWorkflowJob().GetRunID()),
		)
	}
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
// the numbered occurrence. The second return value reports whether any two
// steps shared the same raw name; callers use this to warn operators when
// UseCheckRunID is enabled and the duplicates would cause colliding step span
// IDs.
func newUniqueSteps(steps []*github.TaskStep) ([]string, bool) {
	if len(steps) == 0 {
		return nil, false
	}

	results := make([]string, len(steps))

	count := make(map[string]int, len(steps))
	for _, step := range steps {
		count[step.GetName()]++
	}

	hasDup := false
	occurrences := make(map[string]int, len(steps))
	for i, step := range steps {
		name := step.GetName()
		if count[name] == 1 {
			results[i] = name
			continue
		}

		hasDup = true
		occurrences[name]++
		if occurrences[name] == 1 {
			results[i] = name
		} else {
			results[i] = fmt.Sprintf("%s-%d", name, occurrences[name]-1)
		}
	}

	return results, hasDup
}

// createStepSpan creates a span with a deterministic spandID for the provided
// step.
func (*githubTracesReceiver) createStepSpan(
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
	span.SetKind(ptrace.SpanKindInternal)
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	runID := event.GetWorkflowJob().GetRunID()
	runAttempt := int(event.GetWorkflowJob().GetRunAttempt())
	jobName := event.GetWorkflowJob().GetName()
	stepName := step.GetName()
	number := int(step.GetNumber())

	var spanID pcommon.SpanID
	var err error
	if metadata.ReceiverGithubreceiverUseCheckRunIDFeatureGate.IsEnabled() {
		checkRunID := event.GetWorkflowJob().GetID()
		if checkRunID == 0 {
			return fmt.Errorf(
				"failed to derive step span ID; workflow_job.id is missing for: repo=%q job=%q run_id=%d run_attempt=%d",
				event.GetRepo().GetFullName(),
				event.GetWorkflowJob().GetName(),
				event.GetWorkflowJob().GetRunID(),
				event.GetWorkflowJob().GetRunAttempt(),
			)
		}
		input := fmt.Sprintf("%d-%s-s", checkRunID, step.GetName())
		spanID, err = newSpanIDFromCheckRun(input)
	} else {
		spanID, err = newStepSpanID(runID, runAttempt, jobName, stepName, number)
	}
	if err != nil {
		return fmt.Errorf("failed to generate step span ID: %w", err)
	}

	span.SetSpanID(spanID)

	attrs := span.Attributes()
	attrs.PutStr(string(conventions.CICDPipelineTaskNameKey), name)
	attrs.PutStr(AttributeCICDPipelineTaskRunStatus, step.GetStatus())
	startTime, endTime := correctActionTimestamps(step.GetStartedAt().Time, step.GetCompletedAt().Time)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

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
//
// Deprecated: superseded by newSpanIDFromCheckRun, which is used when the
// receiver.githubreceiver.UseCheckRunID feature gate is enabled (the default as
// of v0.151.0). This function and its callers are scheduled for removal when
// the gate is promoted to Stable. See
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/44856.
func newStepSpanID(runID int64, runAttempt int, jobName, stepName string, number int) (pcommon.SpanID, error) {
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

// newSpanIDFromCheckRun creates span IDs from input. input is a formatted
// string based off GitHub information with an appended value to enable the
// creation of deterministic IDs.
//
// input is as follows:
// jobSpanID := fmt.Sprintf("%d-j", checkRunID)
// queueJobSpanID := fmt.Sprintf("%d-q", checkRunID)
// stepSpanID := fmt.Sprintf("%d-%s-s", checkRunID, stepName)
func newSpanIDFromCheckRun(input string) (pcommon.SpanID, error) {
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
func (*githubTracesReceiver) createJobQueueSpan(
	resourceSpans ptrace.ResourceSpans,
	event *github.WorkflowJobEvent,
	traceID pcommon.TraceID,
	parentSpanID pcommon.SpanID,
) error {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	jobName := event.GetWorkflowJob().GetName()
	spanName := fmt.Sprintf("queue-%s", jobName)

	span.SetName(spanName)
	span.SetKind(ptrace.SpanKindInternal)
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	runID := event.GetWorkflowJob().GetRunID()
	runAttempt := int(event.GetWorkflowJob().GetRunAttempt())

	var spanID pcommon.SpanID
	var err error
	if metadata.ReceiverGithubreceiverUseCheckRunIDFeatureGate.IsEnabled() {
		checkRunID := event.GetWorkflowJob().GetID()
		if checkRunID == 0 {
			return fmt.Errorf(
				"failed to derive queue span ID; workflow_job.id is missing for: repo=%q job=%q run_id=%d run_attempt=%d",
				event.GetRepo().GetFullName(),
				event.GetWorkflowJob().GetName(),
				event.GetWorkflowJob().GetRunID(),
				event.GetWorkflowJob().GetRunAttempt(),
			)
		}
		input := fmt.Sprintf("%d-q", checkRunID)
		spanID, err = newSpanIDFromCheckRun(input)
	} else {
		spanID, err = newStepSpanID(runID, runAttempt, jobName, spanName, 1)
	}
	if err != nil {
		return fmt.Errorf("failed to generate queue span ID: %w", err)
	}

	span.SetSpanID(spanID)

	createdTime, startedTime := correctActionTimestamps(event.GetWorkflowJob().GetCreatedAt().Time, event.GetWorkflowJob().GetStartedAt().Time)
	duration := startedTime.Sub(createdTime)

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(createdTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(startedTime))

	attrs := span.Attributes()
	attrs.PutDouble(AttributeCICDPipelineRunQueueDuration, float64(duration.Nanoseconds()))

	return nil
}
