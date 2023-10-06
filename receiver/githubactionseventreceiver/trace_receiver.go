// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionseventreceiver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
	"go.uber.org/zap"
)

type githubActionsEventReceiver struct {
	nextConsumer    consumer.Traces
	config          *Config
	server          *http.Server
	shutdownWG      sync.WaitGroup
	createSettings  receiver.CreateSettings
	logger          *zap.Logger
	jsonUnmarshaler *jsonTracesUnmarshaler
}

type jsonTracesUnmarshaler struct {
	logger *zap.Logger
}

func (j *jsonTracesUnmarshaler) UnmarshalTraces(blob []byte) (ptrace.Traces, error) {
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
		j.logger.Info("Unmarshalling WorkflowJobEvent")
		traces = eventToTraces(&jobEvent, j.logger)
	} else if _, ok := event["workflow_run"]; ok {
		var runEvent WorkflowRunEvent
		err := json.Unmarshal(blob, &runEvent)
		if err != nil {
			j.logger.Error("Failed to unmarshal run event", zap.Error(err))
			return ptrace.Traces{}, err
		}
		j.logger.Info("Unmarshalling WorkflowRunEvent")
		traces = eventToTraces(&runEvent, j.logger)
	} else {
		j.logger.Error("Unknown event type")
		return ptrace.Traces{}, fmt.Errorf("unknown event type")
	}

	return traces, nil
}

func eventToTraces(event interface{}, logger *zap.Logger) ptrace.Traces {
	logger.Info("Determining event")
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	jobResource := resourceSpans.Resource()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	switch e := event.(type) {
	case *WorkflowJobEvent:
		logger.Info("Processing WorkflowJobEvent")
		createResourceAttributes(jobResource, e, logger)
		processSteps(scopeSpans, e.WorkflowJob.Steps, e.WorkflowJob, logger)
	case *WorkflowRunEvent:
		logger.Info("Processing WorkflowRunEvent")
		// TODO(krzko): Similar logic for WorkflowJobEvent
	default:
		logger.Error("unknown event type")
	}

	return traces
}

func createResourceAttributes(resource pcommon.Resource, event *WorkflowJobEvent, logger *zap.Logger) {
	attrs := resource.Attributes()
	serviceName := fmt.Sprintf("github.%s", strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(event.Repository.FullName, "/", "."), "-", "_")))
	attrs.PutStr("service.name", serviceName)
	attrs.PutStr("service.version", "v0.0.1")

	attrs.PutStr("github.workflow", event.WorkflowJob.Name)
	attrs.PutInt("github.job_id", event.WorkflowJob.ID)
	attrs.PutStr("github.job.html_url", event.WorkflowJob.HTMLURL)
	attrs.PutStr("github.sha", event.WorkflowJob.HeadSha)
	attrs.PutInt("github.job.run_attempt", int64(event.WorkflowJob.RunAttempt))
}

func processSteps(scopeSpans ptrace.ScopeSpans, steps []Step, job WorkflowJob, logger *zap.Logger) {
	if job.Status != "completed" {
		logger.Info("Job not completed, skipping")
		return
	}

	traceID := generateTraceID()
	parentSpanID := createParentSpan(scopeSpans, job, traceID, logger)
	for _, step := range steps {
		createSpan(scopeSpans, step, traceID, parentSpanID, logger)
	}
}

func createParentSpan(scopeSpans ptrace.ScopeSpans, job WorkflowJob, traceID pcommon.TraceID, logger *zap.Logger) pcommon.SpanID {
	logger.Info("Creating parent span", zap.String("name", job.Name))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(generateSpanID())
	span.SetName(job.Name)
	span.SetKind(ptrace.SpanKindServer)
	setSpanTimes(span, job.CreatedAt, job.CompletedAt)
	return span.SpanID()
}

func createSpan(scopeSpans ptrace.ScopeSpans, step Step, traceID pcommon.TraceID, parentSpanID pcommon.SpanID, logger *zap.Logger) pcommon.SpanID {
	logger.Info("Processing span", zap.String("step_name", step.Name))
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetParentSpanID(parentSpanID)
	span.SetSpanID(generateSpanID())
	setSpanTimes(span, step.StartedAt, step.CompletedAt)
	span.SetName(step.Name)
	span.SetKind(ptrace.SpanKindServer)

	if step.Status == "completed" {
		switch step.Conclusion {
		case "success":
			span.Status().SetCode(ptrace.StatusCodeOk)
		case "failure":
			span.Status().SetCode(ptrace.StatusCodeError)
		default:
			span.Status().SetCode(ptrace.StatusCodeUnset)
		}
	} else {
		span.Status().SetCode(ptrace.StatusCodeUnset)
	}
	span.Status().SetMessage(step.Conclusion)

	return span.SpanID()
}

func generateTraceID() pcommon.TraceID {
	var traceID pcommon.TraceID
	binary.Read(rand.Reader, binary.BigEndian, &traceID)
	return traceID
}

func generateSpanID() pcommon.SpanID {
	var spanID pcommon.SpanID
	binary.Read(rand.Reader, binary.BigEndian, &spanID)
	return spanID
}

func setSpanTimes(span ptrace.Span, start, end time.Time) {
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
}

func newReceiver(
	config *Config,
	params receiver.CreateSettings,
	nextConsumer consumer.Traces,
) (*githubActionsEventReceiver, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	gaer := &githubActionsEventReceiver{
		nextConsumer:   nextConsumer,
		config:         config,
		createSettings: params,
		logger:         params.Logger,
		jsonUnmarshaler: &jsonTracesUnmarshaler{
			logger: params.Logger,
		},
	}

	return gaer, nil
}

func (gaer *githubActionsEventReceiver) Start(ctx context.Context, host component.Host) error {
	gaer.server = &http.Server{
		Addr:    gaer.config.HTTPServerSettings.Endpoint,
		Handler: gaer,
	}

	gaer.shutdownWG.Add(1)
	go func() {
		defer gaer.shutdownWG.Done()
		if err := gaer.server.ListenAndServe(); err != http.ErrServerClosed {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (gaer *githubActionsEventReceiver) Shutdown(ctx context.Context) error {
	var err error
	if gaer.server != nil {
		err = gaer.server.Close()
	}
	gaer.shutdownWG.Wait()
	return err
}

func (gaer *githubActionsEventReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.URL.Path != gaer.config.Path {
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
	if gaer.config.Secret != "" {
		receivedSig := r.Header.Get("X-Hub-Signature-256")[7:] // trim off 'sha256=' prefix
		computedHash := hmac.New(sha256.New, []byte(gaer.config.Secret))
		computedHash.Write(slurp)
		expectedSig := hex.EncodeToString(computedHash.Sum(nil))

		if !hmac.Equal([]byte(expectedSig), []byte(receivedSig)) {
			http.Error(w, "Signature mismatch", http.StatusUnauthorized)
			return
		}
	}

	gaer.logger.Debug("Received request", zap.ByteString("payload", slurp))

	td, err := gaer.jsonUnmarshaler.UnmarshalTraces(slurp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gaer.logger.Info("Unmarshaled spans", zap.Int("#spans", td.SpanCount()))

	// Pass the traces to the nextConsumer
	consumerErr := gaer.nextConsumer.ConsumeTraces(ctx, td)
	if consumerErr != nil {
		gaer.logger.Error("Failed to process traces", zap.Error(consumerErr))
		http.Error(w, "Failed to process traces", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
