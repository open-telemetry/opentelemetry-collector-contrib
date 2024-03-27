// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func eventToTraces(event interface{}, config *Config, logger *zap.Logger) (ptrace.Traces, error) {
	logger.Debug("Determining event")
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	switch e := event.(type) {
	case *WorkflowJobEvent:
		logger.Info("Processing WorkflowJobEvent", zap.String("job_name", e.WorkflowJob.Name), zap.String("repo", e.Repository.FullName))
		jobResource := resourceSpans.Resource()
		createResourceAttributes(jobResource, e, config, logger)

		traceID, err := generateTraceID(e.WorkflowJob.RunID, e.WorkflowJob.RunAttempt)
		if err != nil {
			logger.Error("Failed to generate trace ID", zap.Error(err))
			return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
		}

		if e.WorkflowJob.Status == "completed" {
			parentSpanID := createParentSpan(scopeSpans, e.WorkflowJob.Steps, e.WorkflowJob, traceID, logger)
			processSteps(scopeSpans, e.WorkflowJob.Steps, e.WorkflowJob, traceID, parentSpanID, logger)
		}

	case *WorkflowRunEvent:
		logger.Info("Processing WorkflowRunEvent", zap.String("workflow_name", e.WorkflowRun.Name), zap.String("repo", e.Repository.FullName))
		runResource := resourceSpans.Resource()

		traceID, err := generateTraceID(e.WorkflowRun.ID, e.WorkflowRun.RunAttempt)
		if err != nil {
			logger.Error("Failed to generate trace ID", zap.Error(err))
			return ptrace.Traces{}, fmt.Errorf("failed to generate trace ID: %w", err)
		}

		if e.WorkflowRun.Status == "completed" {
			createResourceAttributes(runResource, e, config, logger)
			createRootSpan(resourceSpans, e, traceID, logger)
		}

	default:
		logger.Error("unknown event type, dropping payload")
		return ptrace.Traces{}, fmt.Errorf("unknown event type")
	}

	return traces, nil
}

func createParentSpan(scopeSpans ptrace.ScopeSpans, steps []Step, job WorkflowJob, traceID pcommon.TraceID, logger *zap.Logger) pcommon.SpanID {
	logger.Debug("Creating parent span", zap.String("name", job.Name))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	parentSpanID, _ := generateParentSpanID(job.RunID, job.RunAttempt)
	span.SetParentSpanID(parentSpanID)

	jobSpanID, _ := generateJobSpanID(job.RunID, job.RunAttempt, job.Name)
	span.SetSpanID(jobSpanID)

	span.SetName(job.Name)
	span.SetKind(ptrace.SpanKindServer)
	if len(steps) > 0 {
		setSpanTimes(span, steps[0].StartedAt, steps[len(steps)-1].CompletedAt)
	} else {
		// Invoked when status skipped or cancelled
		logger.Warn("No steps found, defaulting to job times")
		setSpanTimes(span, job.StartedAt, job.CompletedAt)
	}

	allSuccessful := true
	anyFailure := false
	for _, step := range steps {
		if step.Status != "completed" || step.Conclusion != "success" {
			allSuccessful = false
		}
		if step.Conclusion == "failure" {
			anyFailure = true
			break
		}
	}

	if anyFailure {
		span.Status().SetCode(ptrace.StatusCodeError)
	} else if allSuccessful {
		span.Status().SetCode(ptrace.StatusCodeOk)
	} else {
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(job.Conclusion)

	return span.SpanID()
}

func checkDuplicateStepNames(steps []Step) map[string]int {
	nameCount := make(map[string]int)
	for _, step := range steps {
		nameCount[step.Name]++
	}
	return nameCount
}

func convertPRURL(apiURL string) string {
	apiURL = strings.Replace(apiURL, "/repos", "", 1)
	apiURL = strings.Replace(apiURL, "/pulls", "/pull", 1)
	return strings.Replace(apiURL, "api.", "", 1)
}

func createRootSpan(resourceSpans ptrace.ResourceSpans, event *WorkflowRunEvent, traceID pcommon.TraceID, logger *zap.Logger) (pcommon.SpanID, error) {
	logger.Debug("Creating root parent span", zap.String("name", event.WorkflowRun.Name))
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()

	rootSpanID, err := generateParentSpanID(event.WorkflowRun.ID, event.WorkflowRun.RunAttempt)
	if err != nil {
		logger.Error("Failed to generate root span ID", zap.Error(err))
		return pcommon.SpanID{}, fmt.Errorf("failed to generate root span ID: %w", err)
	}

	span.SetTraceID(traceID)
	span.SetSpanID(rootSpanID)
	span.SetName(event.WorkflowRun.Name)
	span.SetKind(ptrace.SpanKindServer)
	setSpanTimes(span, event.WorkflowRun.RunStartedAt, event.WorkflowRun.UpdatedAt)

	switch event.WorkflowRun.Conclusion {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(event.WorkflowRun.Conclusion)

	// Attempt to link to previous trace ID if applicable
	if event.WorkflowRun.PreviousAttemptURL != "" && event.WorkflowRun.RunAttempt > 1 {
		logger.Debug("Linking to previous trace ID for WorkflowRunEvent")
		previousRunAttempt := event.WorkflowRun.RunAttempt - 1
		previousTraceID, err := generateTraceID(event.WorkflowRun.ID, previousRunAttempt)
		if err != nil {
			logger.Error("Failed to generate previous trace ID", zap.Error(err))
		} else {
			link := span.Links().AppendEmpty()
			link.SetTraceID(previousTraceID)
			logger.Debug("Successfully linked to previous trace ID", zap.String("previousTraceID", previousTraceID.String()))
		}
	}

	return rootSpanID, nil
}

func createSpan(scopeSpans ptrace.ScopeSpans, step Step, job WorkflowJob, traceID pcommon.TraceID, parentSpanID pcommon.SpanID, logger *zap.Logger, stepNumber ...int) pcommon.SpanID {
	logger.Debug("Processing span", zap.String("step_name", step.Name))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)

	var spanID pcommon.SpanID

	span.Attributes().PutStr("ci.github.workflow.job.step.name", step.Name)
	span.Attributes().PutStr("ci.github.workflow.job.step.status", step.Status)
	span.Attributes().PutStr("ci.github.workflow.job.step.conclusion", step.Conclusion)
	if len(stepNumber) > 0 && stepNumber[0] > 0 {
		spanID, _ = generateStepSpanID(job.RunID, job.RunAttempt, job.Name, step.Name, stepNumber[0])
		span.Attributes().PutInt("ci.github.workflow.job.step.number", int64(stepNumber[0]))
	} else {
		spanID, _ = generateStepSpanID(job.RunID, job.RunAttempt, job.Name, step.Name)
		span.Attributes().PutInt("ci.github.workflow.job.step.number", int64(step.Number))
	}
	span.Attributes().PutStr("ci.github.workflow.job.step.started_at", step.StartedAt.Format(time.RFC3339))
	span.Attributes().PutStr("ci.github.workflow.job.step.completed_at", step.CompletedAt.Format(time.RFC3339))

	span.SetSpanID(spanID)

	setSpanTimes(span, step.StartedAt, step.CompletedAt)
	span.SetName(step.Name)
	span.SetKind(ptrace.SpanKindServer)

	switch step.Conclusion {
	case "success":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure":
		span.Status().SetCode(ptrace.StatusCodeError)
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}

	span.Status().SetMessage(step.Conclusion)

	return span.SpanID()
}

func generateTraceID(runID int64, runAttempt int) (pcommon.TraceID, error) {
	input := fmt.Sprintf("%d%dt", runID, runAttempt)
	hash := sha256.Sum256([]byte(input))
	traceIDHex := hex.EncodeToString(hash[:])

	var traceID pcommon.TraceID
	_, err := hex.Decode(traceID[:], []byte(traceIDHex[:32]))
	if err != nil {
		return pcommon.TraceID{}, err
	}

	return traceID, nil
}

func generateJobSpanID(runID int64, runAttempt int, job string) (pcommon.SpanID, error) {
	input := fmt.Sprintf("%d%d%s", runID, runAttempt, job)
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func generateParentSpanID(runID int64, runAttempt int) (pcommon.SpanID, error) {
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

func generateServiceName(config *Config, fullName string) string {
	if config.CustomServiceName != "" {
		return config.CustomServiceName
	}
	formattedName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(fullName, "/", "-"), "_", "-"))
	return fmt.Sprintf("%s%s%s", config.ServiceNamePrefix, formattedName, config.ServiceNameSuffix)
}

func generateStepSpanID(runID int64, runAttempt int, jobName, stepName string, stepNumber ...int) (pcommon.SpanID, error) {
	var input string
	if len(stepNumber) > 0 && stepNumber[0] > 0 {
		input = fmt.Sprintf("%d%d%s%s%d", runID, runAttempt, jobName, stepName, stepNumber[0])
	} else {
		input = fmt.Sprintf("%d%d%s%s", runID, runAttempt, jobName, stepName)
	}
	hash := sha256.Sum256([]byte(input))
	spanIDHex := hex.EncodeToString(hash[:])

	var spanID pcommon.SpanID
	_, err := hex.Decode(spanID[:], []byte(spanIDHex[16:32]))
	if err != nil {
		return pcommon.SpanID{}, err
	}

	return spanID, nil
}

func processSteps(scopeSpans ptrace.ScopeSpans, steps []Step, job WorkflowJob, traceID pcommon.TraceID, parentSpanID pcommon.SpanID, logger *zap.Logger) {
	nameCount := checkDuplicateStepNames(steps)
	for index, step := range steps {
		if nameCount[step.Name] > 1 {
			createSpan(scopeSpans, step, job, traceID, parentSpanID, logger, index+1) // Pass step number if duplicate names exist
		} else {
			createSpan(scopeSpans, step, job, traceID, parentSpanID, logger)
		}
	}
}

func setSpanTimes(span ptrace.Span, start, end time.Time) {
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
}

func transformGitHubAPIURL(apiURL string) string {
	htmlURL := strings.Replace(apiURL, "api.github.com/repos", "github.com", 1)
	return htmlURL
}

func validateSignatureSHA256(secret string, signatureHeader string, body []byte, logger *zap.Logger) bool {
	if signatureHeader == "" || len(signatureHeader) < 7 {
		logger.Debug("Unauthorized - No Signature Header")
		return false
	}
	receivedSig := signatureHeader[7:]
	computedHash := hmac.New(sha256.New, []byte(secret))
	computedHash.Write(body)
	expectedSig := hex.EncodeToString(computedHash.Sum(nil))

	logger.Debug("Debugging Signatures", zap.String("Received", receivedSig), zap.String("Computed", expectedSig))

	return hmac.Equal([]byte(expectedSig), []byte(receivedSig))
}

func validateSignatureSHA1(secret string, signatureHeader string, body []byte, logger *zap.Logger) bool {
	if signatureHeader == "" {
		logger.Debug("Unauthorized - No Signature Header")
		return false
	}
	receivedSig := signatureHeader[5:] // Assume "sha1=" prefix
	computedHash := hmac.New(sha1.New, []byte(secret))
	computedHash.Write(body)
	expectedSig := hex.EncodeToString(computedHash.Sum(nil))

	logger.Debug("Debugging Signatures", zap.String("Received", receivedSig), zap.String("Computed", expectedSig))

	return hmac.Equal([]byte(expectedSig), []byte(receivedSig))
}
