// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
)

var (
	conversionDropAttrOpt = metric.WithAttributeSet(attribute.NewSet(attribute.String("stage", "conversion")))
	gatherDropAttrOpt     = metric.WithAttributeSet(attribute.NewSet(attribute.String("stage", "gather")))
	scrapeSuccessAttrOpt  = metric.WithAttributeSet(attribute.NewSet(attribute.String("outcome", "success")))
	scrapeFailureAttrOpt  = metric.WithAttributeSet(attribute.NewSet(attribute.String("outcome", "failure")))
)

type prometheusExporter struct {
	config       Config
	name         string
	endpoint     string
	shutdownFunc func(ctx context.Context) error
	handler      http.Handler
	collector    *collector
	registry     *prometheus.Registry
	telemetry    *metadata.TelemetryBuilder
	settings     component.TelemetrySettings
	stopCh       chan struct{} // signals the background metric cleanup goroutine to stop
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
	writeErr    bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.statusCode = http.StatusOK
		w.wroteHeader = true
	}
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		w.writeErr = true
	}
	return n, err
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func countExportedSeries(mfs []*dto.MetricFamily, req *http.Request) int64 {
	var metricFilter map[string]struct{}
	if req.URL != nil {
		if metricNames := req.URL.Query()["name[]"]; len(metricNames) > 0 {
			metricFilter = make(map[string]struct{}, len(metricNames))
			for _, name := range metricNames {
				metricFilter[name] = struct{}{}
			}
		}
	}
	var exported int64
	for _, mf := range mfs {
		if metricFilter != nil {
			if _, ok := metricFilter[mf.GetName()]; !ok {
				continue
			}
		}
		exported += int64(len(mf.GetMetric()))
	}
	return exported
}

func countGatherErrors(err error) int64 {
	if err == nil {
		return 0
	}
	if multiErr, ok := errors.AsType[prometheus.MultiError](err); ok {
		return int64(len(multiErr))
	}
	return 1
}

func newScrapeHandler(gatherer prometheus.Gatherer, opts promhttp.HandlerOpts, telemetry *metadata.TelemetryBuilder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		var exported int64

		reqGatherer := prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
			mfs, err := gatherer.Gather()
			exported = countExportedSeries(mfs, r)
			if err != nil {
				if dropped := countGatherErrors(err); dropped > 0 {
					telemetry.ExporterPrometheusScrapeDroppedSeries.Add(ctx, dropped, gatherDropAttrOpt)
				}
			}
			return mfs, err
		})

		srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		promhttp.HandlerFor(reqGatherer, opts).ServeHTTP(srw, r)

		failed := srw.statusCode >= http.StatusBadRequest || srw.writeErr
		attrOpt := scrapeSuccessAttrOpt
		if failed {
			attrOpt = scrapeFailureAttrOpt
		} else {
			telemetry.ExporterPrometheusScrapeExportedSeries.Add(ctx, exported)
		}
		telemetry.ExporterPrometheusScrapeDuration.Record(ctx, time.Since(start).Seconds(), attrOpt)
	})
}

var errBlankPrometheusAddress = errors.New("expecting a non-blank address to run the Prometheus metrics handler")

func newPrometheusExporter(config *Config, set exporter.Settings) (*prometheusExporter, error) {
	addr := strings.TrimSpace(config.ServerConfig.NetAddr.Endpoint)
	if strings.TrimSpace(config.ServerConfig.NetAddr.Endpoint) == "" {
		return nil, errBlankPrometheusAddress
	}

	// Return error early because newCollector
	// will call logger.Error if it fails to build the namespace.
	if set.Logger == nil {
		return nil, errors.New("nil logger")
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	collector := newCollector(config, set.Logger, telemetryBuilder)
	if err := telemetryBuilder.RegisterExporterPrometheusActiveSeriesCallback(
		func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(collector.accumulator.activeSeries())
			return nil
		},
	); err != nil {
		telemetryBuilder.Shutdown()
		return nil, err
	}

	registry := prometheus.NewRegistry()
	_ = registry.Register(collector)
	return &prometheusExporter{
		config:       *config,
		name:         set.ID.String(),
		endpoint:     addr,
		collector:    collector,
		registry:     registry,
		telemetry:    telemetryBuilder,
		shutdownFunc: func(_ context.Context) error { return nil },
		handler: newScrapeHandler(
			registry,
			promhttp.HandlerOpts{
				ErrorHandling:     promhttp.ContinueOnError,
				ErrorLog:          newPromLogger(set.Logger),
				EnableOpenMetrics: config.EnableOpenMetrics,
			},
			telemetryBuilder,
		),
		settings: set.TelemetrySettings,
	}, nil
}

func (pe *prometheusExporter) Start(ctx context.Context, host component.Host) error {
	ln, err := pe.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe.handler)
	srv, err := pe.config.ServerConfig.ToServer(ctx, host.GetExtensions(), pe.settings, mux)
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

	// Start a background goroutine that periodically evicts expired metric families.
	// Without this, cleanup only happens during Collect() (i.e. when Prometheus scrapes).
	// If no scraper is active, stale entries in metricFamilies accumulate indefinitely,
	// causing unbounded memory growth. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/41123
	if pe.collector.metricExpiration > 0 {
		pe.stopCh = make(chan struct{})
		go func(stopCh chan struct{}) {
			ticker := time.NewTicker(pe.collector.metricExpiration)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					pe.collector.accumulator.cleanupExpired()
					pe.collector.cleanupMetricFamilies()
				case <-stopCh:
					return
				}
			}
		}(pe.stopCh)
	}

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
	if pe.telemetry != nil {
		pe.telemetry.Shutdown()
	}
	if pe.stopCh != nil {
		close(pe.stopCh)
		pe.stopCh = nil
	}
	return pe.shutdownFunc(ctx)
}
