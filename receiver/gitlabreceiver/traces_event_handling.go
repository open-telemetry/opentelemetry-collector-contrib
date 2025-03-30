package gitlabreceiver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

const (
	gitlabEventTimeFormat = "2006-01-02 15:04:05 UTC"
	gitlabPipelineName    = "Pipeline: %s - %s"
	gitlabJobName         = "Job: %s - %s - Stage: %s"
)

type GitlabEvent interface {
	setAttributes(ptrace.Span) error
	setSpanID(ptrace.Span, pcommon.SpanID) error
	setTimeStamps(ptrace.Span, string, string) error
}

func (gtr *gitlabTracesReceiver) handlePipeline(e *gitlab.PipelineEvent) (ptrace.Traces, error) {
	t := ptrace.NewTraces()
	r := t.ResourceSpans().AppendEmpty()
	r.Resource().Attributes().PutStr(semconv.AttributeServiceName, e.Project.PathWithNamespace)
	pipelineEvent := &GitlabPipelineEvent{e}

	traceId, err := newTraceID(e.ObjectAttributes.ID)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to generate root span ID: %w", err)
	}

	//Pipeline Root Span
	parentSpanID, err := newParentSpanID(e.ObjectAttributes.ID)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to generate parent span ID: %w", err)
	}

	err = gtr.createSpan(r, pipelineEvent, traceId, parentSpanID)
	if err != nil {
		return ptrace.Traces{}, fmt.Errorf("failed to create root span: %w", err)
	}

	for _, job := range e.Builds {
		jobEvent := GitlabPipelineJobEvent(job)
		if job.FinishedAt != "" {
			err := gtr.createSpan(r, &jobEvent, traceId, parentSpanID)
			if err != nil {
				return ptrace.Traces{}, fmt.Errorf("failed to create job span: %w", err)
			}

		}
	}

	return t, nil
}

func (gtr *gitlabTracesReceiver) createSpan(resourceSpans ptrace.ResourceSpans, e GitlabEvent, traceID pcommon.TraceID, parentSpanID pcommon.SpanID) error {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	span.SetTraceID(traceID)

	err := e.setSpanID(span, parentSpanID)
	if err != nil {
		return fmt.Errorf("failed to set span ID: %w", err)
	}

	err = e.setAttributes(span)
	if err != nil {
		return fmt.Errorf("failed to set span attributes: %w", err)
	}

	return nil
}

// newTraceID creates a deterministic Trace ID based on the provided inputs of
// pipelineID. `t` is appended to the end of the input to
// differentiate between a deterministic traceID and the parentSpanID.
func newTraceID(pipelineID int) (pcommon.TraceID, error) {
	input := fmt.Sprintf("%dt", pipelineID)
	hash := sha256.Sum256([]byte(input))
	idHex := hex.EncodeToString(hash[:])

	var id pcommon.TraceID
	_, err := hex.Decode(id[:], []byte(idHex[:32]))
	if err != nil {
		return pcommon.TraceID{}, err
	}

	return id, nil
}

// newParentSpanID creates a deterministic Parent Span ID based on the provided
// pipelineID. `s` is appended to the end of the input to
// differentiate between a deterministic traceID and the parentSpanID.
func newParentSpanID(pipelineID int) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%ds", pipelineID, 0)
	hash := sha256.Sum256([]byte(input))

	var spanID [8]byte
	copy(spanID[:], hash[:8])

	return pcommon.SpanID(spanID), nil
}

// creates a deterministic Job Span ID based on the provided jobId
func newJobSpanID(jobId int) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d", jobId)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func newTimestampFromGitlabTime(t string) (pcommon.Timestamp, error) {
	if t == "" || t == "null" {
		return 0, errors.New("time is empty")
	}

	//For some reason the gitlab test pipeline event has a different time format which we need to support to test (and eventually reenable webhooks) therefoe we are continuing on error to handle the webhook test and the actual webhook
	pt, err := time.Parse(gitlabEventTimeFormat, t)
	if err == nil {
		return pcommon.NewTimestampFromTime(pt), nil
	}

	pt, err = time.Parse(time.RFC3339, t) //Time format of test pipeline events
	if err == nil {
		return pcommon.NewTimestampFromTime(pt), nil
	}

	// pt, err = time.Parse(gitlabEventTimeFormat, t)
	// if err != nil {
	// 	return 0, fmt.Errorf("failed to parse time: %w", err)
	// }

	//This return reflects the error case, not the expected case like usually
	return 0, err
}
