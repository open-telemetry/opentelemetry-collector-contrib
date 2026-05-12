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
	cfg      *Config
	settings receiver.Settings
	nextLogs consumer.Logs

	server     *http.Server
	shutdownWG sync.WaitGroup
}

func newFunctionsReceiver(cfg *Config, settings receiver.Settings, nextLogs consumer.Logs) receiver.Logs {
	return &functionsReceiver{
		cfg:      cfg,
		settings: settings,
		nextLogs: nextLogs,
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
	if t == nil || t.EventHub == nil || len(t.EventHub.Logs) == 0 {
		return nil
	}
	eh := t.EventHub

	unmarshalers, err := loadLogsUnmarshalers(host, eh.Logs)
	if err != nil {
		return err
	}

	decoder := transport.NewBinaryDecoder()
	var extractor MetadataExtractor
	if eh.IncludeMetadata {
		extractor = eventhub.ExtractMetadata
	}

	for _, b := range eh.Logs {
		u := unmarshalers[b.Name]
		protocol := newInvokeProtocol(decoder, r.settings.Logger, extractor)
		consumer := eventhub.NewLogsConsumer(u, r.nextLogs)
		prof := newProfile(b.Name, protocol, consumer)
		path := "/" + b.Name
		mux.Handle(path, createHandler(prof))
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
