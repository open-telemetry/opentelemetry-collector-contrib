// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func newRemoteWriteReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	return &prometheusRemoteWriteReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       cfg,
		server: &http.Server{
			ReadTimeout: 60 * time.Second,
		},
	}, nil
}

type prometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	nextConsumer consumer.Metrics

	config *Config
	server *http.Server
}

func (prw *prometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/write", prw.handlePRW)
	var err error

	prw.server, err = prw.config.ToServer(ctx, host, prw.settings.TelemetrySettings, mux)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	listener, err := prw.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create prometheus remote-write listener: %w", err)
	}

	go func() {
		if err := prw.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting prometheus remote-write receiver: %w", err)))
		}
	}()
	return nil
}

func (prw *prometheusRemoteWriteReceiver) Shutdown(ctx context.Context) error {
	if prw.server == nil {
		return nil
	}
	return prw.server.Shutdown(ctx)
}

func (prw *prometheusRemoteWriteReceiver) handlePRW(_ http.ResponseWriter, _ *http.Request) {
	fmt.Println("handleWrite called")
}
