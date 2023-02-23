// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package websocketprocessor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Processor struct {
	server    *http.Server
	handler   *httpHandler
	logger    *zap.Logger
	marshaler *pmetric.JSONMarshaler
	cs        *channelSet
	consumer  consumer.Metrics
	listener  net.Listener
	limiter   *rate.Limiter
	cfg       *Config
	name      string
}

// newProdMetricsProcessor creates a metrics processor for production
// use (as opposed to for testing).
func newProdMetricsProcessor(
	_ context.Context,
	settings processor.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (processor.Metrics, error) {
	cfg := config.(*Config)
	addr := fmt.Sprintf(":%d", cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on address: %s", addr)
	}
	return newMetricsProcessor(settings, consumer, listener, cfg)
}

// newMetricsProcessor creates a metrics processor where you can pass in a
// listener (useful for testing).
func newMetricsProcessor(settings processor.CreateSettings, consumer consumer.Metrics, listener net.Listener, cfg *Config) (processor.Metrics, error) {
	cs := newChannelSet()
	handler := &httpHandler{
		logger: settings.Logger,
		cs:     cs,
	}
	return &Processor{
		logger:    settings.Logger,
		name:      settings.ID.Name(),
		cfg:       cfg,
		cs:        cs,
		marshaler: &pmetric.JSONMarshaler{},
		handler:   handler,
		consumer:  consumer,
		server:    &http.Server{Handler: handler},
		listener:  listener,
		limiter:   rate.NewLimiter(cfg.Limit, 1),
	}, nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (p *Processor) Start(ctx context.Context, host component.Host) error {
	extensions := host.GetExtensions()
	for k, v := range extensions {
		if k.Type() == "websocketviewer" {
			ext := v.(interface{ RegisterWebsocketProcessor(string, int, rate.Limit) })
			ext.RegisterWebsocketProcessor(p.name, p.cfg.Port, p.cfg.Limit)
		}
	}
	go p.startServer()
	return nil
}

func (p *Processor) startServer() {
	err := p.server.Serve(p.listener) // <- blocks until an error occurs
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		p.logger.Error("server error", zap.Error(err))
	}
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if p.limiter.Allow() {
		err := p.writeToWebsockets(md)
		if err != nil {
			return err
		}
	}
	return p.consumer.ConsumeMetrics(ctx, md)
}

func (p *Processor) writeToWebsockets(md pmetric.Metrics) error {
	bytes, err := p.marshaler.MarshalMetrics(md)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}
	p.cs.writeBytes(bytes)
	return nil
}

func (p *Processor) Shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}
