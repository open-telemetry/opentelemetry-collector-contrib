// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/proto"
)

// MetricType represents supported Prometheus metric types
type MetricType string

const (
	TypeCounter         MetricType = "counter"
	TypeGauge           MetricType = "gauge"
	TypeSummary         MetricType = "summary"
	TypeHistogram       MetricType = "histogram"
	TypeNativeHistogram MetricType = "native_histogram"
)

// MetricDefinition defines a metric and its time series
type MetricDefinition struct {
	Name            string
	Type            MetricType
	Help            string
	Labels          []string
	CreateCollector func() (prometheus.Collector, error)
	Update          func(prometheus.Collector, time.Time) error
}

type Metric struct {
	Collector prometheus.Collector
	Update    func(prometheus.Collector, time.Time) error
}

// Generator manages the metrics and their generation
type Generator struct {
	metrics  map[string]Metric
	registry *prometheus.Registry
	mu       sync.RWMutex
	rand     *rand.Rand
}

var _ http.Handler = (*Generator)(nil)

// NewGenerator creates a new metrics generator
func NewGenerator() *Generator {
	return &Generator{
		metrics:  make(map[string]Metric),
		registry: prometheus.NewRegistry(),
		rand:     rand.New(rand.NewPCG(0xFEEDBEEF, 0xFEEDBEEF)), // for deterministic results
	}
}

// AddMetric adds a new metric definition to the generator
func (g *Generator) AddMetric(def MetricDefinition) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var collector prometheus.Collector
	var err error
	if def.CreateCollector != nil {
		collector, err = def.CreateCollector()
	} else {
		collector, err = g.defaultCollector(def)
	}
	if err != nil {
		return fmt.Errorf("unable to create collector: %w", err)
	}

	if err := g.registry.Register(collector); err != nil {
		return fmt.Errorf("failed to register metric: %w", err)
	}

	g.metrics[def.Name] = Metric{
		Collector: collector,
		Update:    def.Update,
	}
	return nil
}

// UpdateMetrics updates metric values based on the current timestamp
func (g *Generator) UpdateMetrics(timestamp time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, m := range g.metrics {
		if m.Update != nil {
			if err := m.Update(m.Collector, timestamp); err != nil {
				return fmt.Errorf("failed to update metric %s: %w", name, err)
			}
		}
	}

	return nil
}

// ServeHTTP implements http.Handler
func (g *Generator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	format := r.Header.Get("Accept")
	log.Printf("receiver %s request\n", format)
	switch format {
	case "application/json":
		g.serveJSON(w, r)
	case "application/vnd.google.protobuf":
		g.serveProtobuf(w, r)
	default:
		promhttp.HandlerFor(g.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	}
}

// serveJSON serves metrics in JSON format
func (g *Generator) serveJSON(w http.ResponseWriter, _ *http.Request) {
	metrics, err := g.registry.Gather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(metrics)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// serveProtobuf serves metrics in Protobuf format
func (g *Generator) serveProtobuf(w http.ResponseWriter, _ *http.Request) {
	metrics, err := g.registry.Gather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.google.protobuf")

	// Write each MetricFamily as a separate protobuf message
	for _, metricFamily := range metrics {
		data, err := proto.Marshal(metricFamily)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (g *Generator) defaultCollector(def MetricDefinition) (prometheus.Collector, error) {
	switch def.Type {
	case TypeCounter:
		return prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: def.Name,
				Help: def.Help,
			},
			def.Labels,
		), nil

	case TypeGauge:
		return prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: def.Name,
				Help: def.Help,
			},
			def.Labels,
		), nil

	case TypeHistogram:
		return prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: def.Name,
				Help: def.Help,
			},
			def.Labels,
		), nil

	case TypeSummary:
		return prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       def.Name,
				Help:       def.Help,
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			def.Labels,
		), nil

	case TypeNativeHistogram:
		return prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:                         def.Name,
				Help:                         def.Help,
				NativeHistogramBucketFactor:  1.1,
				NativeHistogramZeroThreshold: 1e-6,
			},
			def.Labels,
		), nil
	default:
		return nil, fmt.Errorf("unsupported metric type: %s", def.Type)
	}
}

// GammaRandom generates a random number from a Gamma distribution
// shape (k) and scale (theta) are the parameters
func GammaRandom(rand *rand.Rand, shape, scale float64) float64 {
	// Implementation of Marsaglia and Tsang's method
	if shape < 1 {
		// Use transformation for shape < 1
		return GammaRandom(rand, shape+1, scale) * math.Pow(rand.Float64(), 1.0/shape)
	}

	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		var x, v float64
		for {
			x = rand.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}

		v = v * v * v
		u := rand.Float64()

		if u < 1.0-0.331*math.Pow(x, 4) {
			return d * v * scale
		}

		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v * scale
		}
	}
}
