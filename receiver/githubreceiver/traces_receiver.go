// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

const (
	healthyResponse = `{"text": "GitHub receiver webhook is healthy"}`

	COMPLETED = "completed"
)

type githubTracesReceiver struct {
	traceConsumer consumer.Traces
	cfg           *Config
	server        *http.Server
	shutdownWG    sync.WaitGroup
	settings      receiver.Settings
	logger        *zap.Logger
	obsrecv       *receiverhelper.ObsReport
	ghClient      *github.Client
}

func newTracesReceiver(
	params receiver.Settings,
	config *Config,
	traceConsumer consumer.Traces,
) (*githubTracesReceiver, error) {
	if config.WebHook.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if config.WebHook.TLSSetting != nil {
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

	client := github.NewClient(nil)

	gtr := &githubTracesReceiver{
		traceConsumer: traceConsumer,
		cfg:           config,
		settings:      params,
		logger:        params.Logger,
		obsrecv:       obsrecv,
		ghClient:      client,
	}

	return gtr, nil
}

func (gtr *githubTracesReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitHub WebHook receiving server", zap.String("endpoint", endpoint))

	// noop if not nil. if start has not been called before these values should be nil.
	if gtr.server != nil && gtr.server.Handler != nil {
		return nil
	}

	// create listener from config
	ln, err := gtr.cfg.WebHook.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}

	// use gorilla mux to set up a router
	router := mux.NewRouter()

	// setup health route
	router.HandleFunc(gtr.cfg.WebHook.HealthPath, gtr.handleHealthCheck)
	router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleWebhook)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ServerConfig.ToServer(ctx, host, gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}

	gtr.logger.Info("Health check now listening at", zap.String("health_path", gtr.cfg.WebHook.HealthPath))

	gtr.shutdownWG.Add(1)
	go func() {
		defer gtr.shutdownWG.Done()

		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()

	return nil
}

func (gtr *githubTracesReceiver) Shutdown(_ context.Context) error {
	// server must exist to be closed.
	if gtr.server == nil {
		return nil
	}

	err := gtr.server.Close()
	gtr.shutdownWG.Wait()
	return err
}

// Simple healthcheck endpoint.
func (gtr *githubTracesReceiver) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(healthyResponse))
}

func (gtr *githubTracesReceiver) handleWebhook(w http.ResponseWriter, req *http.Request) {
	var secret []byte
	if gtr.cfg.WebHook.Secret != "" {
		secret = []byte(gtr.cfg.WebHook.Secret)
	}
	payload, err := github.ValidatePayload(req, secret)
	if err != nil {
		gtr.logger.Error("failed to validate the payload", zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to validate the payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	webhookType := github.WebHookType(req)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		gtr.logger.Error("failed to parse webhook", zap.Error(err), zap.String("webhookType", webhookType))
		http.Error(w, fmt.Sprintf("failed to parse the webhook with type '%s': %s", webhookType, err.Error()), http.StatusInternalServerError)
		return
	}
	switch event := event.(type) {
	case *github.WorkflowRunEvent:
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return
	case *github.WorkflowJobEvent:
		if err := gtr.handleWorkflowJobEvent(req.Context(), event); err != nil {
			gtr.logger.Error("failed to handle", zap.Error(err), zap.String("webhookType", webhookType))
			http.Error(w, fmt.Sprintf("failed to handle %s: %s", webhookType, err.Error()), http.StatusInternalServerError)
			return
		} else {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			return

		}
	case *github.PingEvent:
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return
	default:
		gtr.logger.Error("unsupported webhook type", zap.String("webhookType", webhookType))
		http.Error(w, fmt.Sprintf("unsupported webhook type: %s", webhookType), http.StatusInternalServerError)
		return
	}
}

func (gtr *githubTracesReceiver) handleWorkflowJobEvent(ctx context.Context, event *github.WorkflowJobEvent) error {
	if event.Action == nil {
		return nil
	}

	if *event.Action != COMPLETED {
		return nil
	}

	var err error
	job := event.WorkflowJob
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	var traceID pcommon.TraceID
	var spanID pcommon.SpanID
	_, err = rand.Read(spanID[:])
	if err != nil {
		return fmt.Errorf("failed to generate root span id: %w", err)
	}
	_, err = rand.Read(traceID[:])
	if err != nil {
		return fmt.Errorf("failed to generate root trace id: %w", err)
	}

	rootSpan := createSpan(job)
	rootSpan.SetTraceID(traceID)
	rootSpan.SetSpanID(spanID)
	rootSpan.CopyTo(scopeSpans.Spans().AppendEmpty())

	var stepErrors error
	for _, step := range job.Steps {
		if step == nil {
			continue
		}
		_, err := rand.Read(spanID[:])
		if err != nil {
			stepErrors = errors.Join(stepErrors, fmt.Errorf("failed to generate span id: %w", err))
		}

		childSpan := createSpan(step)
		childSpan.SetTraceID(traceID)
		childSpan.SetSpanID(spanID)
		childSpan.SetParentSpanID(rootSpan.SpanID())
		childSpan.CopyTo(scopeSpans.Spans().AppendEmpty())
	}

	if stepErrors != nil {
		return stepErrors
	}

	return gtr.traceConsumer.ConsumeTraces(ctx, traces)
}

func createSpan(step SpanConverter) ptrace.Span {
	span := ptrace.NewSpan()

	name := step.GetName()
	if name != "" {
		span.SetName(name)
	} else {
		span.SetName("unknown")
	}

	startedAt := step.GetStartedAt()
	if !startedAt.IsZero() {
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(startedAt.Time))
	} else {
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}

	completedAt := step.GetCompletedAt()
	if !completedAt.IsZero() {
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(completedAt.Time))
	} else {
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}

	conclusion := step.GetConclusion()
	switch conclusion {
	case "success", "cancelled":
		span.Status().SetCode(ptrace.StatusCodeOk)
	case "failure", "timed_out":
		span.Status().SetCode(ptrace.StatusCodeError)
	}
	return span
}

type SpanConverter interface {
	GetConclusion() string
	GetCompletedAt() github.Timestamp
	GetStartedAt() github.Timestamp
	GetName() string
}
