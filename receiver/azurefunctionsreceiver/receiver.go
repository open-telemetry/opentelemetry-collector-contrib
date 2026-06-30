// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/eventhub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/transport"
)

type functionsReceiver struct {
	cfg         *Config
	settings    receiver.Settings
	nextLogs    consumer.Logs
	nextMetrics consumer.Metrics

	server     *http.Server
	shutdownWG sync.WaitGroup
}

func newFunctionsReceiver(cfg *Config, settings receiver.Settings) *functionsReceiver {
	return &functionsReceiver{
		cfg:      cfg,
		settings: settings,
	}
}

func (r *functionsReceiver) Start(ctx context.Context, host component.Host) error {
	if r.server != nil {
		return nil
	}

	mux := http.NewServeMux()
	if err := r.registerTriggerRoutes(mux, host); err != nil {
		return err
	}

	server, err := r.cfg.HTTP.ToServer(ctx, host.GetExtensions(), r.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}
	r.server = server

	listener, err := r.cfg.HTTP.ToListener(ctx)
	if err != nil {
		return err
	}

	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", r.cfg.HTTP.NetAddr.Endpoint))
	r.shutdownWG.Go(func() {
		if errHTTP := r.server.Serve(listener); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			r.settings.Logger.Error("HTTP server error", zap.Error(errHTTP))
		}
	})

	return nil
}

// registerTriggerRoutes attaches HTTP handlers for each configured trigger
func (r *functionsReceiver) registerTriggerRoutes(mux *http.ServeMux, host component.Host) error {
	return r.registerEventHubRoutes(mux, host)
}

func (r *functionsReceiver) registerEventHubRoutes(mux *http.ServeMux, host component.Host) error {
	t := r.cfg.Triggers
	if t == nil || t.EventHub == nil {
		return nil
	}
	if len(t.EventHub.Logs) == 0 && len(t.EventHub.Metrics) == 0 {
		return nil
	}
	eh := t.EventHub

	decoder := transport.NewBinaryDecoder()
	var extractor MetadataExtractor
	if eh.IncludeMetadata {
		extractor = eventhub.ExtractMetadata
	}

	if r.nextLogs != nil && len(eh.Logs) > 0 {
		unmarshalers, err := loadLogsUnmarshalers(host, eh.Logs)
		if err != nil {
			return err
		}
		for _, b := range eh.Logs {
			u := unmarshalers[b.Name]
			protocol := newInvokeProtocol(decoder, r.settings.Logger, extractor)
			cons := eventhub.NewLogsConsumer(u, r.nextLogs)
			prof := newProfile(b.Name, protocol, cons)
			mux.Handle("/"+b.Name, createHandler(prof))
		}
	}

	if r.nextMetrics != nil && len(eh.Metrics) > 0 {
		unmarshalers, err := loadMetricsUnmarshalers(host, eh.Metrics)
		if err != nil {
			return err
		}
		for _, b := range eh.Metrics {
			u := unmarshalers[b.Name]
			protocol := newInvokeProtocol(decoder, r.settings.Logger, extractor)
			cons := eventhub.NewMetricsConsumer(u, r.nextMetrics)
			prof := newProfile(b.Name, protocol, cons)
			mux.Handle("/"+b.Name, createHandler(prof))
		}
	}

	return nil
}

func (r *functionsReceiver) Shutdown(ctx context.Context) error {
	if r.server == nil {
		return nil
	}
	err := r.server.Shutdown(ctx)
	r.shutdownWG.Wait()
	return err
}
