// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/metrics"
)

// OTLPPublisher handles publishing metrics to OTLP endpoints using telemetrygen
type OTLPPublisher struct {
	endpoint string
}

// NewOTLPPublisher creates a new OTLP publisher using telemetrygen
func NewOTLPPublisher(endpoint string) *OTLPPublisher {
	return &OTLPPublisher{
		endpoint: endpoint,
	}
}

// SendHistogramMetric sends a histogram metric using telemetrygen
func (p *OTLPPublisher) SendHistogramMetric(metricName string, _ HistogramResult) error {
	return p.SendMetric(metricName, "Histogram", 0) // Histogram value doesn't matter for telemetrygen
}

// SendMetric sends a metric using telemetrygen with the specified type and value
func (p *OTLPPublisher) SendMetric(metricName string, metricType string, _ float64) error {
	// Create telemetrygen config
	cfg := metrics.NewConfig()
	cfg.CustomEndpoint = p.endpoint
	cfg.UseHTTP = true
	cfg.Insecure = true
	cfg.NumMetrics = 1
	cfg.Rate = 1
	cfg.MetricName = metricName

	// Set metric type
	switch metricType {
	case "Sum":
		cfg.MetricType = metrics.MetricTypeSum
	case "Gauge":
		cfg.MetricType = metrics.MetricTypeGauge
	case "Histogram":
		cfg.MetricType = metrics.MetricTypeHistogram
	default:
		cfg.MetricType = metrics.MetricTypeGauge // default to gauge
	}

	// Start the metrics generation
	err := metrics.Start(cfg)
	if err != nil {
		return fmt.Errorf("failed to send metric via telemetrygen: %w", err)
	}

	return nil
}

// SendGaugeMetric sends a gauge metric
func (p *OTLPPublisher) SendGaugeMetric(metricName string, value float64) error {
	return p.SendMetric(metricName, "Gauge", value)
}

// SendSumMetric sends a sum/counter metric
func (p *OTLPPublisher) SendSumMetric(metricName string, value float64) error {
	return p.SendMetric(metricName, "Sum", value)
}

// SendHistogramMetricSimple sends a histogram metric (telemetrygen will generate histogram data)
func (p *OTLPPublisher) SendHistogramMetricSimple(metricName string) error {
	return p.SendMetric(metricName, "Histogram", 0)
}
