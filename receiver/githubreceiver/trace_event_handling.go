// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v68/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func (gtr *githubTracesReceiver) handleWorkflowRun(e *github.WorkflowRunEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()

	resource := r.Resource()

	err := gtr.getWorkflowAttrs(resource, e)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to get workflow attributes: %w", err)
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

// TODO: Add and implement handleWorkflowJob, tying corresponding job spans to
// the proper root span and trace ID.

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
