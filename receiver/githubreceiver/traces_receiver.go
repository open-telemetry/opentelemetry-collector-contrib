// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/go-github/v66/github"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

type githubTracesReceiver struct {
	nextConsumer   consumer.Traces
	config         *Config
	server         *http.Server
	shutdownWG     sync.WaitGroup
	createSettings receiver.Settings
	logger         *zap.Logger
	obsrecv        *receiverhelper.ObsReport
	ghClient       *github.Client
}

func newTracesReceiver(
	params receiver.Settings,
	config *Config,
	nextConsumer consumer.Traces,
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
		nextConsumer:   nextConsumer,
		config:         config,
		createSettings: params,
		logger:         params.Logger,
		obsrecv:        obsrecv,
		ghClient:       client,
	}

	return gtr, nil
}

func (gtr *githubTracesReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gtr.config.WebHook.Endpoint, gtr.config.WebHook.Path)
	gtr.logger.Info("Starting GitHub WebHook receiving server", zap.String("endpoint", endpoint))

	// noop if not nil. if start has not been called before these values should be nil.
	if gtr.server != nil && gtr.server.Handler != nil {
		return nil
	}

	//  #nosec G112
	// gtr.server = &http.Server{
	// 	Addr:    gtr.config.WebHook.ServerConfig.Endpoint,
	// 	// Handler: gtr.server.Handler,
	// }

	// create listener from config
	ln, err := gtr.config.WebHook.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}

	// set up router.
	router := httprouter.New()

	// router.POST(gtr.config.WebHook.Path, gtr.handleReq)
	// router.GET(gtr.config.WebHook.HealthPath, gtr.handleHealthCheck)

	// router.POST(gtr.config.WebHook.Path, gtr.handleReq)
	// router.GET(gtr.config.WebHook.HealthPath, gtr.handleHealthCheck)

	// webhook server standup and configuration
	gtr.server, err = gtr.config.WebHook.ServerConfig.ToServer(ctx, host, gtr.createSettings.TelemetrySettings, router)
	if err != nil {
		return err
	}

	// readTimeout, err := time.ParseDuration(gtr.config.WebHook.ReadTimeout)
	// if err != nil {
	// 	return err
	// }
	//
	// writeTimeout, err := time.ParseDuration(er.cfg.WriteTimeout)
	// if err != nil {
	// 	return err
	// }

	// set timeouts
	// er.server.ReadHeaderTimeout = readTimeout
	// er.server.WriteTimeout = writeTimeout

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

// func (gtr *githubTracesReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//
// 	// Validate request path
// 	if r.URL.Path != gtr.config.WebHook.Path {
// 		http.Error(w, "Not found", http.StatusNotFound)
// 		return
// 	}
//
// 	// Validate the payload using the configured secret
// 	payload, err := github.ValidatePayload(r, []byte(gtr.config.WebHook.Secret))
// 	if err != nil {
// 		gtr.logger.Debug("Payload validation failed", zap.Error(err))
// 		http.Error(w, "Invalid payload or signature", http.StatusBadRequest)
// 		return
// 	}
//
// 	// Determine the type of GitHub webhook event and ensure it's one we handle
// 	eventType := github.WebHookType(r)
// 	event, err := github.ParseWebHook(eventType, payload)
// 	if err != nil {
// 		gtr.logger.Debug("Webhook parsing failed", zap.Error(err))
// 		http.Error(w, "Failed to parse webhook", http.StatusBadRequest)
// 		return
// 	}
//
// 	// Handle events based on specific types and completion status
// 	switch e := event.(type) {
// 	case *github.WorkflowJobEvent:
// 		if e.GetWorkflowJob().GetStatus() != "completed" {
// 			gtr.logger.Debug("Skipping non-completed WorkflowJobEvent", zap.String("status", e.GetWorkflowJob().GetStatus()))
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}
// 	case *github.WorkflowRunEvent:
// 		if e.GetWorkflowRun().GetStatus() != "completed" {
// 			gtr.logger.Debug("Skipping non-completed WorkflowRunEvent", zap.String("status", e.GetWorkflowRun().GetStatus()))
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}
// 	default:
// 		gtr.logger.Debug("Skipping unsupported event type", zap.String("event", eventType))
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}
//
// 	gtr.logger.Debug("Received valid GitHub event", zap.String("type", eventType))
//
// 	w.WriteHeader(http.StatusAccepted)
// }
