// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type prometheusExporter struct {
	config       Config
	name         string
	endpoint     string
	shutdownFunc func(ctx context.Context) error
	handler      http.Handler
	collector    *collector
	registry     *prometheus.Registry
	settings     component.TelemetrySettings
}

var errBlankPrometheusAddress = errors.New("expecting a non-blank address to run the Prometheus metrics handler")

func newPrometheusExporter(config *Config, set exporter.Settings) (*prometheusExporter, error) {
	addr := strings.TrimSpace(config.Endpoint)
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errBlankPrometheusAddress
	}

	collector := newCollector(config, set.Logger)
	registry := prometheus.NewRegistry()
	_ = registry.Register(collector)
	return &prometheusExporter{
		config:       *config,
		name:         set.ID.String(),
		endpoint:     addr,
		collector:    collector,
		registry:     registry,
		shutdownFunc: func(_ context.Context) error { return nil },
		handler: promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorHandling:     promhttp.ContinueOnError,
				ErrorLog:          newPromLogger(set.Logger),
				EnableOpenMetrics: config.EnableOpenMetrics,
			},
		),
		settings: set.TelemetrySettings,
	}, nil
}

func (pe *prometheusExporter) Start(ctx context.Context, host component.Host) error {
	ln, err := pe.config.ToListener(ctx)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe.handler)
	srv, err := pe.config.ToServer(ctx, host, pe.settings, mux)
	pe.shutdownFunc = func(ctx context.Context) error {
		errLn := ln.Close()
		if srv == nil {
			return errLn
		}
		errSrv := srv.Shutdown(ctx)
		return errors.Join(errLn, errSrv)
	}
	if err != nil {
		return err
	}
	go func() {
		_ = srv.Serve(ln)
	}()

	return nil
}

func (pe *prometheusExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	n := 0
	rmetrics := md.ResourceMetrics()
	for i := 0; i < rmetrics.Len(); i++ {
		n += pe.collector.processMetrics(rmetrics.At(i))
	}

	return nil
}

func (pe *prometheusExporter) Shutdown(ctx context.Context) error {
	return pe.shutdownFunc(ctx)
}
