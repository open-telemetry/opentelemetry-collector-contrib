// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/go-github/v61/github"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

var errMissingEndpoint = errors.New("missing a receiver endpoint")

type githubActionsReceiver struct {
	nextConsumer   consumer.Traces
	config         *Config
	server         *http.Server
	shutdownWG     sync.WaitGroup
	createSettings receiver.CreateSettings
	logger         *zap.Logger
	obsrecv        *receiverhelper.ObsReport
	ghClient       *github.Client
}

func newTracesReceiver(
	params receiver.CreateSettings,
	config *Config,
	nextConsumer consumer.Traces,
) (*githubActionsReceiver, error) {
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

	client := github.NewClient(nil)

	gar := &githubActionsReceiver{
		nextConsumer:   nextConsumer,
		config:         config,
		createSettings: params,
		logger:         params.Logger,
		obsrecv:        obsrecv,
		ghClient:       client,
	}

	return gar, nil
}

func (gar *githubActionsReceiver) Start(ctx context.Context, host component.Host) error {
	endpoint := fmt.Sprintf("%s%s", gar.config.Endpoint, gar.config.Path)
	gar.logger.Info("Starting GithubActions server", zap.String("endpoint", endpoint))
	gar.server = &http.Server{
		Addr:    gar.config.ServerConfig.Endpoint,
		Handler: gar,
	}

	gar.shutdownWG.Add(1)
	go func() {
		defer gar.shutdownWG.Done()

		if errHTTP := gar.server.ListenAndServe(); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			gar.createSettings.TelemetrySettings.Logger.Error("Server closed with error", zap.Error(errHTTP))
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

	// Validate request path
	if r.URL.Path != gar.config.Path {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Validate the payload using the configured secret
	payload, err := github.ValidatePayload(r, []byte(gar.config.Secret))
	if err != nil {
		gar.logger.Debug("Payload validation failed", zap.Error(err))
		http.Error(w, "Invalid payload or signature", http.StatusBadRequest)
		return
	}

	// Determine the type of GitHub webhook event and ensure it's one we handle
	eventType := github.WebHookType(r)
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		gar.logger.Debug("Webhook parsing failed", zap.Error(err))
		http.Error(w, "Failed to parse webhook", http.StatusBadRequest)
		return
	}

	// Handle events based on specific types and completion status
	switch e := event.(type) {
	case *github.WorkflowJobEvent:
		if e.GetWorkflowJob().GetStatus() != "completed" {
			gar.logger.Debug("Skipping non-completed WorkflowJobEvent", zap.String("status", e.GetWorkflowJob().GetStatus()))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	case *github.WorkflowRunEvent:
		if e.GetWorkflowRun().GetStatus() != "completed" {
			gar.logger.Debug("Skipping non-completed WorkflowRunEvent", zap.String("status", e.GetWorkflowRun().GetStatus()))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	default:
		gar.logger.Debug("Skipping unsupported event type", zap.String("event", eventType))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	gar.logger.Debug("Received valid GitHub event", zap.String("type", eventType))

	// Convert the GitHub event to OpenTelemetry traces
	td, err := eventToTraces(event, gar.config, gar.logger)
	if err != nil {
		gar.logger.Debug("Failed to convert event to traces", zap.Error(err))
		http.Error(w, "Error processing traces", http.StatusBadRequest)
		return
	}

	// Process traces if any are present
	if td.SpanCount() > 0 {
		gar.logger.Debug("Unmarshaled spans", zap.Int("#spans", td.SpanCount()))

		// Pass the traces to the nextConsumer
		consumerErr := gar.nextConsumer.ConsumeTraces(ctx, td)
		if consumerErr != nil {
			gar.logger.Debug("Failed to process traces", zap.Error(consumerErr))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		gar.logger.Debug("No spans to unmarshal or traces not initialised")
	}

	w.WriteHeader(http.StatusAccepted)
}
