// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v81/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

const healthyResponse = `{"text": "GitHub receiver webhook is healthy"}`

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
	if config.WebHook.NetAddr.Endpoint == "" {
		return nil, errMissingEndpoint
	}

	transport := "http"
	if config.WebHook.TLS.HasValue() {
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
	endpoint := fmt.Sprintf("%s%s", gtr.cfg.WebHook.NetAddr.Endpoint, gtr.cfg.WebHook.Path)
	gtr.logger.Info("Starting GitHub WebHook receiving server", zap.String("endpoint", endpoint))

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
	router.HandleFunc(gtr.cfg.WebHook.Path, gtr.handleReq)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ToServer(ctx, host.GetExtensions(), gtr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}

	gtr.logger.Info("Health check now listening at", zap.String("health_path", gtr.cfg.WebHook.HealthPath))

	gtr.shutdownWG.Go(func() {
		if errHTTP := gtr.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	})

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

// handleReq handles incoming request sent to the webhook endpoint. On success
// returns a 200 response code.
func (gtr *githubTracesReceiver) handleReq(w http.ResponseWriter, req *http.Request) {
	ctx := gtr.obsrecv.StartTracesOp(req.Context())

	p, err := github.ValidatePayload(req, []byte(gtr.cfg.WebHook.Secret))
	if err != nil {
		gtr.logger.Sugar().Debugf("unable to validate payload", zap.Error(err))
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	eventType := github.WebHookType(req)
	event, err := github.ParseWebHook(eventType, p)
	if err != nil {
		gtr.logger.Sugar().Debugf("failed to parse event", zap.Error(err))
		http.Error(w, "failed to parse event", http.StatusBadRequest)
		return
	}

	var td ptrace.Traces
	switch e := event.(type) {
	case *github.WorkflowRunEvent:
		if strings.ToLower(e.GetWorkflowRun().GetStatus()) != "completed" {
			gtr.logger.Debug("workflow run not complete, skipping...", zap.String("status", e.GetWorkflowRun().GetStatus()))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		td, err = gtr.handleWorkflowRun(e, p)
	case *github.WorkflowJobEvent:
		if strings.ToLower(e.GetWorkflowJob().GetStatus()) != "completed" {
			gtr.logger.Debug("workflow job not complete, skipping...", zap.String("status", e.GetWorkflowJob().GetStatus()))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		td, err = gtr.handleWorkflowJob(e, p)
	case *github.PingEvent:
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return
	default:
		gtr.logger.Sugar().Debugf("event type not supported", zap.String("event_type", eventType))
		http.Error(w, "event type not supported", http.StatusBadRequest)
		return
	}

	if td.SpanCount() > 0 {
		err = gtr.traceConsumer.ConsumeTraces(ctx, td)
		if err != nil {
			http.Error(w, "failed to consume traces", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	gtr.obsrecv.EndTracesOp(ctx, "protobuf", td.SpanCount(), err)
}

// Simple healthcheck endpoint.
func (*githubTracesReceiver) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(healthyResponse))
}
