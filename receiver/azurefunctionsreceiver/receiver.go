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
	// TODO: Refactor so Start() collects path+profile specs from each trigger (logs and, later, metrics) and registers them in one loop; adding a trigger or signal should not duplicate registration logic here.
	if r.cfg.EventHub != nil && len(r.cfg.EventHub.Logs) > 0 {
		unmarshalers, err := loadLogsUnmarshalers(host, r.cfg.EventHub.Logs)
		if err != nil {
			return err
		}

		decoder := transport.NewBinaryDecoder()
		var extractor MetadataExtractor
		if r.cfg.EventHub.IncludeMetadata {
			extractor = eventhub.ExtractMetadata
		}

		for _, b := range r.cfg.EventHub.Logs {
			u := unmarshalers[b.Name]
			protocol := NewInvokeProtocol(decoder, r.settings.Logger, extractor)
			consumer := eventhub.NewLogsConsumer(u, r.nextLogs)
			profile := NewProfile(b.Name, protocol, consumer)
			path := "/" + b.Name
			mux.Handle(path, createHandler(profile))
		}
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

func (r *functionsReceiver) Shutdown(ctx context.Context) error {
	if r.server == nil {
		return nil
	}
	err := r.server.Shutdown(ctx)
	r.shutdownWG.Wait()
	return err
}
