// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/go-github/v67/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
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
