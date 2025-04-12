// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
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

	gtr := &gitlabTracesReceiver{
		traceConsumer: traceConsumer,
		cfg:           cfg,
		settings:      settings,
		logger:        settings.Logger,
		obsrecv:       obsrecv,
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
	ln, err := gtr.cfg.WebHook.ToListener(ctx)
	if err != nil {
		return err
	}

	// use gorilla mux to set up a router
	router := mux.NewRouter()

	// setup health route
	router.HandleFunc(gtr.cfg.WebHook.HealthPath, gtr.handleHealthCheck)

	// webhook server standup and configuration
	gtr.server, err = gtr.cfg.WebHook.ToServer(ctx, host, gtr.settings.TelemetrySettings, router)
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
