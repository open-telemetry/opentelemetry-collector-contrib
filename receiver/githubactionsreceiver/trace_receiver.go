// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

type githubActionsReceiver struct {
	nextConsumer    consumer.Traces
	config          *Config
	server          *http.Server
	shutdownWG      sync.WaitGroup
	createSettings  receiver.CreateSettings
	logger          *zap.Logger
	jsonUnmarshaler *jsonTracesUnmarshaler
	obsrecv         *receiverhelper.ObsReport
}

type jsonTracesUnmarshaler struct {
	logger *zap.Logger
}

func (j *jsonTracesUnmarshaler) UnmarshalTraces(blob []byte, config *Config) (ptrace.Traces, error) {
	var event map[string]json.RawMessage
	if err := json.Unmarshal(blob, &event); err != nil {
		j.logger.Error("Failed to unmarshal blob", zap.Error(err))
		return ptrace.Traces{}, err
	}

	var traces ptrace.Traces
	if _, ok := event["workflow_job"]; ok {
		var jobEvent WorkflowJobEvent
		err := json.Unmarshal(blob, &jobEvent)
		if err != nil {
			j.logger.Error("Failed to unmarshal job event", zap.Error(err))
			return ptrace.Traces{}, err
		}
		j.logger.Debug("Unmarshalling WorkflowJobEvent")
		traces, err = eventToTraces(&jobEvent, config, j.logger)
		if err != nil {
			j.logger.Error("Failed to convert event to traces", zap.Error(err))
			return ptrace.Traces{}, err
		}
	} else if _, ok := event["workflow_run"]; ok {
		var runEvent WorkflowRunEvent
		err := json.Unmarshal(blob, &runEvent)
		if err != nil {
			j.logger.Error("Failed to unmarshal run event", zap.Error(err))
			return ptrace.Traces{}, err
		}
		j.logger.Debug("Unmarshalling WorkflowRunEvent")
		traces, err = eventToTraces(&runEvent, config, j.logger)
		if err != nil {
			j.logger.Error("Failed to convert event to traces", zap.Error(err))
			return ptrace.Traces{}, err
		}
	} else {
		j.logger.Warn("Unknown event type")
		return ptrace.Traces{}, fmt.Errorf("unknown event type")
	}

	return traces, nil
}

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
			return traces, fmt.Errorf("failed to generate trace ID")
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
			return traces, fmt.Errorf("failed to generate trace ID")
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

	jobSpanID, _ := generateJobSpanID(job.ID, job.RunAttempt, job.Name)
	span.SetSpanID(jobSpanID)

	span.SetName(job.Name)
	span.SetKind(ptrace.SpanKindServer)
	if len(steps) > 0 {
		setSpanTimes(span, steps[0].StartedAt, steps[len(steps)-1].CompletedAt)
	} else {
		logger.Warn("No steps found, defaulting to job times")
		setSpanTimes(span, job.CreatedAt, job.CompletedAt)
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

func createResourceAttributes(resource pcommon.Resource, event interface{}, config *Config, logger *zap.Logger) {
	attrs := resource.Attributes()

	switch e := event.(type) {
	case *WorkflowJobEvent:
		serviceName := generateServiceName(config, e.Repository.FullName)
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.name", e.WorkflowJob.WorkflowName)

		attrs.PutStr("ci.github.workflow.job.created_at", e.WorkflowJob.CreatedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.completed_at", e.WorkflowJob.CompletedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.conclusion", e.WorkflowJob.Conclusion)
		attrs.PutStr("ci.github.workflow.job.head_branch", e.WorkflowJob.HeadBranch)
		attrs.PutStr("ci.github.workflow.job.head_sha", e.WorkflowJob.HeadSha)
		attrs.PutStr("ci.github.workflow.job.html_url", e.WorkflowJob.HTMLURL)
		attrs.PutInt("ci.github.workflow.job.id", e.WorkflowJob.ID)
		if len(e.WorkflowJob.Labels) > 0 {
			joinedLabels := strings.Join(e.WorkflowJob.Labels, ",")
			attrs.PutStr("ci.github.workflow.job.labels", joinedLabels)
		} else {
			attrs.PutStr("ci.github.workflow.job.labels", "no labels")
		}
		attrs.PutStr("ci.github.workflow.job.name", e.WorkflowJob.Name)
		attrs.PutInt("ci.github.workflow.job.run_attempt", int64(e.WorkflowJob.RunAttempt))
		attrs.PutInt("ci.github.workflow.job.run_id", e.WorkflowJob.RunID)
		attrs.PutStr("ci.github.workflow.job.runner.group_name", e.WorkflowJob.RunnerGroupName)
		attrs.PutStr("ci.github.workflow.job.runner.name", e.WorkflowJob.RunnerName)
		attrs.PutStr("ci.github.workflow.job.sender.login", e.Sender.Login)
		attrs.PutStr("ci.github.workflow.job.started_at", e.WorkflowJob.StartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.status", e.WorkflowJob.Status)

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.git.repo.owner.login", e.Repository.Owner.Login)
		attrs.PutStr("scm.git.repo", e.Repository.FullName)

	case *WorkflowRunEvent:
		serviceName := generateServiceName(config, e.Repository.FullName)
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.run.actor.login", e.WorkflowRun.Actor.Login)

		attrs.PutStr("ci.github.workflow.run.conclusion", e.WorkflowRun.Conclusion)
		attrs.PutStr("ci.github.workflow.run.created_at", e.WorkflowRun.CreatedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.display_title", e.WorkflowRun.DisplayTitle)
		attrs.PutStr("ci.github.workflow.run.event", e.WorkflowRun.Event)
		attrs.PutStr("ci.github.workflow.run.head_branch", e.WorkflowRun.HeadBranch)
		attrs.PutStr("ci.github.workflow.run.head_sha", e.WorkflowRun.HeadSha)
		attrs.PutStr("ci.github.workflow.run.html_url", e.WorkflowRun.HTMLURL)
		attrs.PutInt("ci.github.workflow.run.id", e.WorkflowRun.ID)
		attrs.PutStr("ci.github.workflow.run.name", e.WorkflowRun.Name)
		attrs.PutStr("ci.github.workflow.run.path", e.WorkflowRun.Path)
		if e.WorkflowRun.PreviousAttemptURL != "" {
			attrs.PutStr("ci.github.workflow.run.previous_attempt_url", e.WorkflowRun.PreviousAttemptURL)
		}
		attrs.PutInt("ci.github.workflow.run.run_attempt", int64(e.WorkflowRun.RunAttempt))
		attrs.PutStr("ci.github.workflow.run.run_started_at", e.WorkflowRun.RunStartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.status", e.WorkflowRun.Status)
		attrs.PutStr("ci.github.workflow.run.updated_at", e.WorkflowRun.UpdatedAt.Format(time.RFC3339))

		attrs.PutStr("ci.github.workflow.run.sender.login", e.Sender.Login)
		attrs.PutStr("ci.github.workflow.run.triggering_actor.login", e.WorkflowRun.TriggeringActor.Login)
		attrs.PutStr("ci.github.workflow.run.updated_at", e.WorkflowRun.UpdatedAt.Format(time.RFC3339))

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.system", "git")

		attrs.PutStr("scm.git.head_branch", e.WorkflowRun.HeadBranch)
		attrs.PutStr("scm.git.head_commit.author.email", e.WorkflowRun.HeadCommit.Author.Email)
		attrs.PutStr("scm.git.head_commit.author.name", e.WorkflowRun.HeadCommit.Author.Name)
		attrs.PutStr("scm.git.head_commit.committer.email", e.WorkflowRun.HeadCommit.Committer.Email)
		attrs.PutStr("scm.git.head_commit.committer.name", e.WorkflowRun.HeadCommit.Committer.Name)
		attrs.PutStr("scm.git.head_commit.message", e.WorkflowRun.HeadCommit.Message)
		attrs.PutStr("scm.git.head_commit.timestamp", e.WorkflowRun.HeadCommit.Timestamp.Format(time.RFC3339))
		attrs.PutStr("scm.git.head_sha", e.WorkflowRun.HeadSha)

		if len(e.WorkflowRun.PullRequests) > 0 {
			var prUrls []string
			for _, pr := range e.WorkflowRun.PullRequests {
				prUrls = append(prUrls, convertPRURL(pr.URL))
			}
			attrs.PutStr("scm.git.pull_requests.url", strings.Join(prUrls, ";"))
		}

		attrs.PutStr("scm.git.repo", e.Repository.FullName)

	default:
		logger.Error("unknown event type")
	}
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
		return pcommon.SpanID{}, err
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

func newTracesReceiver(
	params receiver.CreateSettings,
	config *Config,
	nextConsumer consumer.Traces,
) (*githubActionsReceiver, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	if config.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if config.TLSSetting != nil {
		transport = "https"
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              transport,
		ReceiverCreateSettings: params,
	})

	if err != nil {
		return nil, err
	}

	gar := &githubActionsReceiver{
		nextConsumer:   nextConsumer,
		config:         config,
		createSettings: params,
		logger:         params.Logger,
		jsonUnmarshaler: &jsonTracesUnmarshaler{
			logger: params.Logger,
		},
		obsrecv: obsrecv,
	}

	return gar, nil
}

func (gar *githubActionsReceiver) Start(ctx context.Context, host component.Host) error {
	endpint := fmt.Sprintf("%s%s", gar.config.Endpoint, gar.config.Path)
	gar.logger.Info("Starting GithubActions server", zap.String("endpoint", endpint))
	gar.server = &http.Server{
		Addr:    gar.config.HTTPServerSettings.Endpoint,
		Handler: gar,
	}

	gar.shutdownWG.Add(1)
	go func() {
		defer gar.shutdownWG.Done()
		if err := gar.server.ListenAndServe(); err != http.ErrServerClosed {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (gar *githubActionsReceiver) Shutdown(ctx context.Context) error {
	var err error
	if gar.server != nil {
		err = gar.server.Close()
	}
	gar.shutdownWG.Wait()
	return err
}

func (gar *githubActionsReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userAgent := r.Header.Get("User-Agent")
	if !strings.HasPrefix(userAgent, "GitHub-Hookshot") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.URL.Path != gar.config.Path {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	defer r.Body.Close()

	slurp, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// Validate the request if Secret is set in the configuration
	if gar.config.Secret != "" {
		signatureSHA256 := r.Header.Get("X-Hub-Signature-256")
		if signatureSHA256 != "" && !validateSignatureSHA256(gar.config.Secret, signatureSHA256, slurp, gar.logger) {
			gar.logger.Debug("Unauthorized - Signature Mismatch SHA256")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			signatureSHA1 := r.Header.Get("X-Hub-Signature")
			if signatureSHA1 != "" && !validateSignatureSHA1(gar.config.Secret, signatureSHA1, slurp, gar.logger) {
				gar.logger.Debug("Unauthorized - Signature Mismatch SHA1")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	gar.logger.Debug("Received request", zap.ByteString("payload", slurp))

	td, err := gar.jsonUnmarshaler.UnmarshalTraces(slurp, gar.config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gar.logger.Debug("Unmarshaled spans", zap.Int("#spans", td.SpanCount()))

	// Pass the traces to the nextConsumer
	consumerErr := gar.nextConsumer.ConsumeTraces(ctx, td)
	if consumerErr != nil {
		gar.logger.Error("Failed to process traces", zap.Error(consumerErr))
		http.Error(w, "Failed to process traces", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
