// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheustracemetricsexporter

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
)

type (
	exporter struct {
		cfg *Config

		spanCounter   prometheus.Counter
		spanBatchSize prometheus.Gauge

		marshaler pdata.TracesMarshaler

		serverStopper serverStopper
	}

	serverStopper struct {
		lock sync.Mutex
		cb   func() error
	}
)

const (
	metricsNamespace = "promtracemetrics"
)

var (
	_ component.TracesExporter = &exporter{}
)

func (ss *serverStopper) Set(cb func() error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	ss.cb = cb
}

func (ss *serverStopper) Stop() error {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if ss.cb == nil {
		return nil
	}
	return ss.cb()
}

func newExporter(cfg *Config) *exporter {
	e := &exporter{
		cfg:       cfg,
		marshaler: otlp.NewProtobufTracesMarshaler(),
	}
	e.createPromCounters()

	return e
}

func (e *exporter) Start(_ context.Context, _ component.Host) error {
	prometheus.MustRegister(e.spanCounter)
	prometheus.MustRegister(e.spanBatchSize)

	go func() {
		server := &http.Server{
			Addr: e.cfg.ScrapeListenAddr,
			Handler: promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{
					// EnableOpenMetrics: true,
				},
			)}

		err := server.ListenAndServe()
		if err != nil {
			log.Printf("HTTP server (%s) listen failed: %s", metricsNamespace, err)
		} else {
			e.serverStopper.Set(server.Close)
		}
	}()

	return nil
}

func (e *exporter) Shutdown(_ context.Context) error {
	err := e.serverStopper.Stop()
	if err != nil {
		log.Printf("Could not stop HTTP server (%s) properly: %s", metricsNamespace, err)
	}

	prometheus.Unregister(e.spanCounter)
	prometheus.Unregister(e.spanBatchSize)

	return nil
}

func (e *exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *exporter) ConsumeTraces(_ context.Context, td pdata.Traces) error {
	raw, err := e.marshaler.MarshalTraces(td)
	if err != nil {
		log.Printf("Could not marshal traces: %s", err)
		return nil
	}

	e.spanCounter.Add(float64(td.SpanCount()))
	e.spanBatchSize.Add(float64(len(raw)))

	return nil
}

func (e *exporter) createPromCounters() {
	e.spanCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "span_count",
	})

	e.spanBatchSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "span_batch_size",
	})
}
