// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
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
	transport := "http"
	if cfg.WebHook.TLSSetting != nil {
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
		return nil, err
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
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitLab WebHook receiving server", zap.String("endpoint", endpoint))

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

	// setup webhook route for traces
	router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleWebhook)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ServerConfig.ToServer(ctx, host, gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}
	gtr.logger.Info("Health check now listening at", zap.String("health_path", fmt.Sprintf("%s%s", gtr.cfg.WebHook.Endpoint, gtr.cfg.WebHook.HealthPath)))

	gtr.shutdownWG.Add(1)
	go func() {
		defer gtr.shutdownWG.Done()
		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()

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

// Simple healthcheck endpoint.
func (gtr *gitlabTracesReceiver) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(healthyResponse))
}

// handleReq handles incoming request sent to the webhook endoint
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
		gtr.failBadReq(ctx, w, http.StatusBadRequest, fmt.Errorf("unexpected event type: %T", event), 0)
		return
	}

	if e.ObjectAttributes.FinishedAt == "" {
		gtr.logger.Debug("pipeline not complete, skipping...", zap.String("status", e.ObjectAttributes.Status))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	traces, err := gtr.handlePipeline(e)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
		return
	}

	spanCount := traces.SpanCount()
	err = gtr.traceConsumer.ConsumeTraces(ctx, traces)
	if err != nil {
		gtr.failBadReq(ctx, w, http.StatusInternalServerError, err, 0)
		return
	}

	w.WriteHeader(http.StatusOK)
	gtr.obsrecv.EndTracesOp(ctx, metadata.Type.String(), spanCount, nil)
}

func (gtr *gitlabTracesReceiver) validateReq(r *http.Request) (gitlab.EventType, error) {
	if r.Method != http.MethodPost {
		return "", errors.New("invalid HTTP method")
	}

	if gtr.cfg.WebHook.Secret != "" {
		secret := r.Header.Get(defaultGitlabTokenHeader)
		if secret != gtr.cfg.WebHook.Secret {
			return "", fmt.Errorf("invalid %s header", defaultGitlabTokenHeader)
		}
	}

	eventType := gitlab.WebhookEventType(r)
	if eventType == "" {
		return "", fmt.Errorf("missing %s header", defaultGitlabEventHeader)
	}
	return eventType, nil
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

	jsonResp, marshalErr := jsoniter.Marshal(err.Error())
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

	gtr.logger.Debug(string(jsonResp), zap.Int("http_status_code", httpStatusCode), zap.Error(err))
}
