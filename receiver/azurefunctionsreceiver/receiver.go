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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/eventhub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/transport"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"
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
	eh := t.EventHub

	decoder := transport.NewBinaryDecoder()
	var extractor MetadataExtractor
	if eh.IncludeMetadata {
		extractor = eventhub.ExtractMetadata
	}

	if r.nextLogs != nil {
		err := registerEventHubSignalRoutes(
			mux, host, eh.Logs, "logs", decoder, r.settings.Logger, extractor,
			func(u plog.Unmarshaler) trigger.Consumer { return eventhub.NewLogsConsumer(u, r.nextLogs) },
		)
		if err != nil {
			return err
		}
	}

	if r.nextMetrics != nil {
		err := registerEventHubSignalRoutes(
			mux, host, eh.Metrics, "metrics", decoder, r.settings.Logger, extractor,
			func(u pmetric.Unmarshaler) trigger.Consumer { return eventhub.NewMetricsConsumer(u, r.nextMetrics) },
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// registerEventHubSignalRoutes registers one HTTP route per Event Hub binding.
func registerEventHubSignalRoutes[T any](
	mux *http.ServeMux,
	host component.Host,
	bindings []EncodingConfig,
	signalType string,
	decoder *transport.BinaryDecoder,
	logger *zap.Logger,
	extractor MetadataExtractor,
	newConsumer func(T) trigger.Consumer,
) error {
	unmarshalers, err := loadUnmarshalers[T](host, bindings, signalType)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		protocol := newInvokeProtocol(decoder, logger, extractor)
		consumer := newConsumer(unmarshalers[b.Name])
		mux.Handle("/"+b.Name, createHandler(newProfile(b.Name, protocol, consumer)))
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
