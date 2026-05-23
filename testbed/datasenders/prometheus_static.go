// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// PrometheusStaticPayloadConfig defines how to populate a prometheus.Registry for benchmarking.
type PrometheusStaticPayloadConfig struct {
	SeriesCount          int
	LabelsPerSeries      int
	WithTargetInfo       bool
	WithNativeHistograms bool
}

// prometheusStaticSender serves a pre-populated Prometheus registry via promhttp.
// The collector's prometheusreceiver scrapes this endpoint.
type prometheusStaticSender struct {
	testbed.DataSenderBase
	server   *http.Server
	registry *prometheus.Registry
}

// NewPrometheusStaticSender builds a registry from the given config and returns
// a DataSender that serves it on the given host:port.
func NewPrometheusStaticSender(host string, port int, cfg PrometheusStaticPayloadConfig) testbed.DataSender {
	reg := buildRegistry(cfg)
	return &prometheusStaticSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
		registry: reg,
	}
}

func (s *prometheusStaticSender) Start() error {
	handler := promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	s.server = &http.Server{
		Addr:              s.GetEndpoint().String(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	go func() { _ = s.server.Serve(ln) }()
	return nil
}

func (s *prometheusStaticSender) Stop() error {
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

func (s *prometheusStaticSender) GenConfigYAMLStr() string {
	format := `
  prometheus:
    config:
      scrape_configs:
        - job_name: 'testbed'
          scrape_interval: 100ms
          static_configs:
            - targets: ['%s']
`
	return fmt.Sprintf(format, s.GetEndpoint())
}

func (*prometheusStaticSender) ProtocolName() string {
	return "prometheus"
}

func buildRegistry(cfg PrometheusStaticPayloadConfig) *prometheus.Registry {
	reg := prometheus.NewRegistry()

	if cfg.WithTargetInfo {
		targetInfo := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "target_info",
			Help: "Target metadata",
			ConstLabels: prometheus.Labels{
				"environment": "test",
				"region":      "us-west-2",
				"cluster":     "benchmark-cluster",
			},
		})
		targetInfo.Set(1)
		reg.MustRegister(targetInfo)
	}

	labelsPerSeries := cfg.LabelsPerSeries
	if labelsPerSeries <= 0 {
		labelsPerSeries = 5
	}

	labelNames := make([]string, labelsPerSeries)
	for i := range labelNames {
		labelNames[i] = fmt.Sprintf("label_%d", i)
	}

	for i := range cfg.SeriesCount {
		name := fmt.Sprintf("metric_%d", i)

		if cfg.WithNativeHistograms {
			h := prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:                            name,
				Help:                            fmt.Sprintf("Benchmark histogram %d", i),
				NativeHistogramBucketFactor:     1.1,
				NativeHistogramMaxBucketNumber:  100,
				NativeHistogramMinResetDuration: 0,
				NativeHistogramMaxZeroThreshold: 0.001,
			})
			h.Observe(float64(i) + 0.5)
			reg.MustRegister(h)
		} else {
			labelValues := make([]string, labelsPerSeries)
			for j := range labelValues {
				labelValues[j] = fmt.Sprintf("value_%d_%d", i, j)
			}
			cv := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: name,
				Help: fmt.Sprintf("Benchmark counter %d", i),
			}, labelNames)
			cv.WithLabelValues(labelValues...).Add(float64(i) + 1)
			reg.MustRegister(cv)
		}
	}

	return reg
}
