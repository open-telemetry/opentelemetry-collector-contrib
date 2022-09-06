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

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type prometheusExporter struct {
	config       Config
	name         string
	endpoint     string
	shutdownFunc func() error
	handler      http.Handler
	collector    *collector
	registry     *prometheus.Registry
	settings     component.TelemetrySettings
}

var errBlankPrometheusAddress = errors.New("expecting a non-blank address to run the Prometheus metrics handler")

func newPrometheusExporter(config *Config, set component.ExporterCreateSettings) (*prometheusExporter, error) {
	addr := strings.TrimSpace(config.Endpoint)
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, errBlankPrometheusAddress
	}

	collector := newCollector(config, set.Logger)
	registry := prometheus.NewRegistry()
	_ = registry.Register(collector)
	return &prometheusExporter{
		config:       *config,
		name:         config.ID().String(),
		endpoint:     addr,
		collector:    collector,
		registry:     registry,
		shutdownFunc: func() error { return nil },
		handler: promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorHandling:     promhttp.ContinueOnError,
				ErrorLog:          newPromLogger(set.Logger),
				EnableOpenMetrics: config.EnableOpenMetrics,
			},
		),
	}, nil
}

func (pe *prometheusExporter) Start(_ context.Context, host component.Host) error {
	ln, err := pe.config.ToListener()
	if err != nil {
		return err
	}

	pe.shutdownFunc = ln.Close

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe.handler)
	srv, err := pe.config.ToServer(host, pe.settings, mux)
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

func (pe *prometheusExporter) Shutdown(context.Context) error {
	return pe.shutdownFunc()
}
