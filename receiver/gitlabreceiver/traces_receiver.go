// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

const (
	// Completed pipeline statuses
	pipelineStatusSuccess  = "success"
	pipelineStatusFailed   = "failed"
	pipelineStatusCanceled = "canceled"
	pipelineStatusSkipped  = "skipped"

	// In-progress pipeline statuses
	pipelineStatusRunning            = "running"
	pipelineStatusPending            = "pending"
	pipelineStatusCreated            = "created"
	pipelineStatusWaitingForResource = "waiting_for_resource"
	pipelineStatusPreparing          = "preparing"
	pipelineStatusScheduled          = "scheduled"
)

var (
	// Error messages
	errMissingEndpoint      = errors.New("missing a receiver endpoint")
	errGitlabClient         = errors.New("failed to create gitlab client")
	errUnexpectedEvent      = errors.New("unexpected event type")
	errInvalidHTTPMethod    = errors.New("invalid HTTP method")
	errInvalidHeader        = errors.New("invalid header")
	errMissingHeader        = errors.New("missing header")
	errMissingRequiredField = errors.New("missing required field")
)

const healthyResponse = `{"text": "GitLab receiver webhook is healthy"}`

type gitlabTracesReceiver struct {
	cfg           *Config
	settings      receiver.Settings
	traceConsumer consumer.Traces
	obsrecv       *receiverhelper.ObsReport
	server        *http.Server
	shutdownWG    sync.WaitGroup
	logger        *zap.Logger
	gitlabClient  *gitlab.Client
}

func newTracesReceiver(settings receiver.Settings, cfg *Config, traceConsumer consumer.Traces) (*gitlabTracesReceiver, error) {
	if cfg.WebHook.NetAddr.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if cfg.WebHook.TLS.HasValue() {
		transport = "https"
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	client, err := gitlab.NewClient("")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGitlabClient, err)
	}

	gtr := &gitlabTracesReceiver{
		traceConsumer: traceConsumer,
		cfg:           cfg,
		settings:      settings,
		logger:        settings.Logger,
		obsrecv:       obsrecv,
		gitlabClient:  client,
	}

	return gtr, nil
}

func (gtr *gitlabTracesReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.NetAddr.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitLab WebHook receiving server", zap.String("endpoint", endpoint))

	// noop if not nil. if start has not been called before these values should be nil.
	if gtr.server != nil && gtr.server.Handler != nil {
		return nil
	}

	// create listener from config
	ln, err := gtr.cfg.WebHook.ToListener(ctx)
	if err != nil {
		return err
	}

	// use gorilla mux to set up a router
	router := mux.NewRouter()

	// setup health route
	router.HandleFunc(gtr.cfg.WebHook.HealthPath, gtr.handleHealthCheck)

	// setup webhook route for traces
	router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleWebhook)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ToServer(ctx, host.GetExtensions(), gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}
	gtr.logger.Info(
		"Health check now listening at",
		zap.String("health_path",
			fmt.Sprintf("%s%s", gtr.cfg.WebHook.NetAddr.Endpoint, gtr.cfg.WebHook.HealthPath),
		),
	)

	gtr.shutdownWG.Go(func() {
		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	})

	return nil
}

func (gtr *gitlabTracesReceiver) Shutdown(ctx context.Context) error {
	if gtr.server != nil {
		err := gtr.server.Shutdown(ctx)
		return err
	}
	gtr.shutdownWG.Wait()
	return nil
}

func (gtr *gitlabTracesReceiver) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		_, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			gtr.failBadReq(r.Context(), w, http.StatusInternalServerError, err, 0)
			return
		}
		_ = r.Body.Close()
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(healthyResponse))
}

// handleReq handles incoming request sent to the webhook endpoint
func (gtr *gitlabTracesReceiver) handleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := gtr.obsrecv.StartTracesOp(r.Context())

	eventType, err := gtr.validateReq(r)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	event, err := gitlab.ParseWebhook(eventType, payload)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	e, ok := event.(*gitlab.PipelineEvent)
	if !ok {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, fmt.Errorf("%w: %T", errUnexpectedEvent, event), 0)
		return
	}

	// Check if the finishedAt timestamp is present, which is required for traceID generation
	if e.ObjectAttributes.FinishedAt == "" {
		gtr.logger.Debug("pipeline missing finishedAt timestamp, skipping...",
			zap.String("status", e.ObjectAttributes.Status))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Process the pipeline based on its status
	switch strings.ToLower(e.ObjectAttributes.Status) {
	case pipelineStatusRunning, pipelineStatusPending, pipelineStatusCreated, pipelineStatusWaitingForResource, pipelineStatusPreparing, pipelineStatusScheduled:
		gtr.logger.Debug("pipeline not complete, skipping...",
			zap.String("status", e.ObjectAttributes.Status))
		w.WriteHeader(http.StatusNoContent)
		return
	case pipelineStatusSuccess, pipelineStatusFailed, pipelineStatusCanceled, pipelineStatusSkipped:
		// above statuses are indicators of a completed pipeline, so we process them
		break
	default:
		gtr.logger.Warn("unknown pipeline status, skipping...",
			zap.String("status", e.ObjectAttributes.Status))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = gtr.validatePipelineEvent(e)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusBadRequest, err, 0)
		return
	}

	traces, err := gtr.handlePipeline(e)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
		return
	}

	if traces.SpanCount() > 0 {
		err = gtr.traceConsumer.ConsumeTraces(ctx, traces)
		if err != nil {
			gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}

	gtr.obsrecv.EndTracesOp(ctx, metadata.Type.String(), traces.SpanCount(), nil)
}

func (gtr *gitlabTracesReceiver) validateReq(r *http.Request) (gitlab.EventType, error) {
	if r.Method != http.MethodPost {
		return "", errInvalidHTTPMethod
	}

	if gtr.cfg.WebHook.Secret != "" {
		secret := r.Header.Get(defaultGitLabSecretTokenHeader)
		if secret != gtr.cfg.WebHook.Secret {
			return "", fmt.Errorf("%w: %s", errInvalidHeader, defaultGitLabSecretTokenHeader)
		}
	}

	for key, value := range gtr.cfg.WebHook.RequiredHeaders {
		if r.Header.Get(key) != string(value) {
			return "", fmt.Errorf("%w: %s", errInvalidHeader, key)
		}
	}

	eventType := gitlab.WebhookEventType(r)
	if eventType == "" {
		return "", fmt.Errorf("%w: %s", errMissingHeader, defaultGitLabEventHeader)
	}
	return eventType, nil
}

// validatePipelineEvent validates critical webhook event fields for trace generation
// The following values should ALWAYS be present in a valid pipeline event
// They are required to set foundational attributes
func (*gitlabTracesReceiver) validatePipelineEvent(e *gitlab.PipelineEvent) error {
	if e.ObjectAttributes.ID == 0 {
		return fmt.Errorf("%w: pipeline ID", errMissingRequiredField)
	}

	if e.ObjectAttributes.CreatedAt == "" {
		return fmt.Errorf("%w: pipeline created_at", errMissingRequiredField)
	}

	if e.ObjectAttributes.Ref == "" {
		return fmt.Errorf("%w: pipeline ref", errMissingRequiredField)
	}

	if e.ObjectAttributes.SHA == "" {
		return fmt.Errorf("%w: pipeline SHA", errMissingRequiredField)
	}

	if e.Project.ID == 0 {
		return fmt.Errorf("%w: project ID", errMissingRequiredField)
	}

	if e.Project.PathWithNamespace == "" {
		return fmt.Errorf("%w: project path_with_namespace", errMissingRequiredField)
	}

	if e.Commit.ID == "" {
		return fmt.Errorf("%w: commit ID", errMissingRequiredField)
	}

	return nil
}

func (gtr *gitlabTracesReceiver) failBadReq(ctx context.Context,
	w http.ResponseWriter,
	httpStatusCode int,
	err error,
	spanCount int,
) {
	defer gtr.obsrecv.EndTracesOp(ctx, metadata.Type.String(), spanCount, err)

	if err == nil {
		w.WriteHeader(httpStatusCode)
		return
	}

	jsonResp, marshalErr := json.Marshal(err.Error())
	if marshalErr != nil {
		gtr.logger.Warn("failed to marshall error to json", zap.Error(marshalErr))
		w.WriteHeader(httpStatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)

	if _, writeErr := w.Write(jsonResp); writeErr != nil {
		gtr.logger.Warn("failed to write json response", zap.Error(writeErr))
	}

	gtr.logger.Debug(string(jsonResp), zap.Int(string(conventions.HTTPResponseStatusCodeKey), httpStatusCode), zap.Error(err))
}
