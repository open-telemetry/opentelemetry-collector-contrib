// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

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

	// background metric cleanup 
	cleanupCancel context.CancelFunc
	cleanupWG     sync.WaitGroup
}

var errBlankPrometheusAddress = errors.New("expecting a non-blank address to run the Prometheus metrics handler")

func newPrometheusExporter(config *Config, set exporter.Settings) (*prometheusExporter, error) {
	addr := strings.TrimSpace(config.Endpoint)
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errBlankPrometheusAddress
	}

	// Return error early because newCollector
	// will call logger.Error if it fails to build the namespace.
	if set.Logger == nil {
		return nil, errors.New("nil logger")
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
	if err != nil {
		lnerr := ln.Close()
		return errors.Join(err, lnerr)
	}
	pe.shutdownFunc = func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
	go func() {
		_ = srv.Serve(ln)
	}()

	interval := pe.config.MetricExpiration / 2
	cleanupCtx, cancel := context.WithCancel(context.Background())
	pe.cleanupCancel = cancel
	pe.cleanupWG.Add(1)
	go func() {
		ticker := time.NewTicker(interval)
		defer pe.cleanupWG.Done()
		defer ticker.Stop()
		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				pe.collector.cleanupMetricFamilies()
			}
		}
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
	if pe.cleanupCancel != nil {
		pe.cleanupCancel()
		done := make(chan struct{})
		go func() {
			// wait for cleanup goroutine to finish
			pe.cleanupWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			// timeout exceeded
		}
	}
	return pe.shutdownFunc(ctx)
}
