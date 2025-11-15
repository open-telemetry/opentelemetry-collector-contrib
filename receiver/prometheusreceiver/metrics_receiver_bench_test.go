// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promslog"
	promcfg "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

// BenchmarkPrometheusReceiver benchmarks the end-to-end performance of the Prometheus receiver
// including HTTP fetch, protocol parsing, and transaction processing.
func BenchmarkPrometheusReceiver(b *testing.B) {
	b.Run("Protocol_PromText", func(b *testing.B) {
		b.Run("ClassicTypes", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:    protocolPromText,
				metricType:  metricTypeClassicTypes,
				seriesCount: 100,
			})
		})

		b.Run("ClassicTypes_WithTargetInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:       protocolPromText,
				metricType:     metricTypeClassicTypes,
				seriesCount:    100,
				withTargetInfo: true,
			})
		})

		b.Run("ClassicTypes_WithScopeInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:      protocolPromText,
				metricType:    metricTypeClassicTypes,
				seriesCount:   100,
				withScopeInfo: true,
			})
		})
	})

	b.Run("Protocol_PromProtobuf", func(b *testing.B) {
		b.Run("ClassicTypes", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:    protocolPromProtobuf,
				metricType:  metricTypeClassicTypes,
				seriesCount: 100,
			})
		})

		b.Run("ClassicTypes_WithTargetInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:       protocolPromProtobuf,
				metricType:     metricTypeClassicTypes,
				seriesCount:    100,
				withTargetInfo: true,
			})
		})

		b.Run("ClassicTypes_WithScopeInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:      protocolPromProtobuf,
				metricType:    metricTypeClassicTypes,
				seriesCount:   100,
				withScopeInfo: true,
			})
		})

		b.Run("NativeHistogram", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:    protocolPromProtobuf,
				metricType:  metricTypeNativeHistogram,
				seriesCount: 100,
			})
		})

		b.Run("NativeHistogram_WithTargetInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:       protocolPromProtobuf,
				metricType:     metricTypeNativeHistogram,
				seriesCount:    100,
				withTargetInfo: true,
			})
		})

		b.Run("NativeHistogram_WithScopeInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:      protocolPromProtobuf,
				metricType:    metricTypeNativeHistogram,
				seriesCount:   100,
				withScopeInfo: true,
			})
		})
	})

	b.Run("Protocol_OpenMetrics", func(b *testing.B) {
		b.Run("ClassicTypes", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:    protocolOpenMetrics,
				metricType:  metricTypeClassicTypes,
				seriesCount: 100,
			})
		})

		b.Run("ClassicTypes_WithTargetInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:       protocolOpenMetrics,
				metricType:     metricTypeClassicTypes,
				seriesCount:    100,
				withTargetInfo: true,
			})
		})

		b.Run("ClassicTypes_WithScopeInfo", func(b *testing.B) {
			benchmarkReceiver(b, benchmarkConfig{
				protocol:      protocolOpenMetrics,
				metricType:    metricTypeClassicTypes,
				seriesCount:   100,
				withScopeInfo: true,
			})
		})
	})
}

type protocol int

const (
	protocolPromText protocol = iota
	protocolPromProtobuf
	protocolOpenMetrics
)

type metricType int

const (
	metricTypeNativeHistogram metricType = iota
	metricTypeClassicTypes               // Mix of all basic metric types (gauge, counter, summary, histogram)
)

// benchmarkConfig holds configuration for a benchmark run.
// It controls the protocol format, metric types, data volume, and special info metrics.
type benchmarkConfig struct {
	protocol       protocol
	metricType     metricType
	seriesCount    int
	withTargetInfo bool
	withScopeInfo  bool
}

// benchmarkReceiver runs a complete benchmark of the Prometheus receiver
func benchmarkReceiver(b *testing.B, cfg benchmarkConfig) {
	metricData := generateMetricData(cfg)
	mockServer := createMockPrometheusServer(metricData, cfg.protocol)
	defer mockServer.Close()

	ctx := b.Context()
	receiver, consumer, cleanup := setupBenchmarkReceiver(b, mockServer.URL, cfg)
	defer cleanup()

	if err := receiver.Start(ctx, componenttest.NewNopHost()); err != nil {
		b.Fatalf("Failed to start receiver: %v", err)
	}
	defer func() {
		if err := receiver.Shutdown(ctx); err != nil {
			b.Errorf("Failed to shutdown receiver: %v", err)
		}
	}()

	// Wait for the first scrape to ensure everything is working
	waitForScrape(b, consumer, 10*time.Second)

	// Reset timer to exclude setup and first scrape time
	b.ResetTimer()
	b.ReportAllocs()

	// Benchmark by counting how many scrapes happen during the benchmark duration
	// The receiver will continue scraping in the background
	initialCount := len(consumer.AllMetrics())

	// Let it run for the benchmark duration
	for i := 0; i < b.N; i++ {
		// Wait for another scrape to complete
		waitForNextScrape(b, consumer, initialCount+i+1, 2*time.Second)
	}
}

// setupBenchmarkReceiver creates and configures a Prometheus receiver for benchmarking
func setupBenchmarkReceiver(b *testing.B, serverURL string, cfg benchmarkConfig) (*pReceiver, *consumertest.MetricsSink, func()) {
	b.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		b.Fatalf("Failed to parse server URL: %v", err)
	}

	job := map[string]any{
		"job_name":        "benchmark",
		"metrics_path":    "/metrics",
		"scrape_interval": "100ms", // Short interval for benchmarking
		"scrape_timeout":  "50ms",  // Must be less than scrape_interval
		"static_configs":  []map[string]any{{"targets": []string{u.Host}}},
	}

	configMap := map[string]any{
		"scrape_configs": []map[string]any{job},
	}

	cfgBytes, err := yaml.Marshal(&configMap)
	if err != nil {
		b.Fatalf("Failed to marshal config: %v", err)
	}

	pCfg, err := promcfg.Load(string(cfgBytes), promslog.NewNopLogger())
	if err != nil {
		b.Fatalf("Failed to load Prometheus config: %v", err)
	}

	receiverCfg := &Config{
		PrometheusConfig:     (*PromConfig)(pCfg),
		StartTimeMetricRegex: "",
	}

	if cfg.metricType == metricTypeNativeHistogram {
		receiverCfg.enableNativeHistograms = true
	}

	consumer := new(consumertest.MetricsSink)

	receiver, err := newPrometheusReceiver(
		receivertest.NewNopSettings(metadata.Type),
		receiverCfg,
		consumer,
	)
	if err != nil {
		b.Fatalf("Failed to create receiver: %v", err)
	}

	receiver.skipOffsetting = true

	cleanup := func() {}

	return receiver, consumer, cleanup
}

// generateMetricData generates Prometheus metric data based on the configuration
// Returns the encoded metric data as a string
func generateMetricData(cfg benchmarkConfig) string {
	registry := prometheus.NewRegistry()

	switch cfg.metricType {
	case metricTypeNativeHistogram:
		createNativeHistogramMetrics(registry, cfg.seriesCount)
	case metricTypeClassicTypes:
		createClassicMetrics(registry, cfg.seriesCount)
	}

	if cfg.withTargetInfo {
		createTargetInfo(registry)
	}

	if cfg.withScopeInfo {
		createScopeInfo(registry)
	}

	return encodeMetrics(registry, cfg.protocol)
}

func createNativeHistogramMetrics(registry *prometheus.Registry, count int) {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:                            "http_request_duration_seconds_native",
			Help:                            "HTTP request duration in seconds (native histogram)",
			NativeHistogramBucketFactor:     1.1, // Base-2 exponential factor
			NativeHistogramMaxBucketNumber:  100,
			NativeHistogramMinResetDuration: 1 * time.Hour,
		},
		[]string{"method"},
	)
	registry.MustRegister(histogram)

	methods := []string{"GET", "POST", "PUT"}

	for i := 0; i < count; i++ {
		method := methods[i%len(methods)]
		for j := 0; j < 100; j++ {
			histogram.WithLabelValues(method).Observe(float64(j) / 100.0)
		}
	}
}

// createClassicMetrics creates a mix of all classic metric types in the registry
func createClassicMetrics(registry *prometheus.Registry, count int) {
	gaugeCount := count / 4
	counterCount := count / 4
	summaryCount := count / 4
	histogramCount := count - (gaugeCount + counterCount + summaryCount)

	if gaugeCount > 0 {
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Active HTTP requests",
			},
			[]string{"method", "status"},
		)
		registry.MustRegister(gauge)

		methods := []string{"GET", "POST", "PUT"}
		statuses := []string{"200", "201", "202"}

		for i := 0; i < gaugeCount; i++ {
			method := methods[i%len(methods)]
			status := statuses[i%len(statuses)]
			gauge.WithLabelValues(method, status).Set(float64(100 + i))
		}
	}

	if counterCount > 0 {
		counter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "status"},
		)
		registry.MustRegister(counter)

		methods := []string{"GET", "POST", "PUT"}
		statuses := []string{"200", "201", "202"}

		for i := 0; i < counterCount; i++ {
			method := methods[i%len(methods)]
			status := statuses[i%len(statuses)]
			counter.WithLabelValues(method, status).Add(float64(1000 + i*10))
		}
	}

	if summaryCount > 0 {
		summary := prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_request_duration_seconds",
				Help:       "HTTP request duration in seconds",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			[]string{"method"},
		)
		registry.MustRegister(summary)

		methods := []string{"GET", "POST", "PUT"}

		for i := 0; i < summaryCount; i++ {
			method := methods[i%len(methods)]
			// Observe some values to populate the summary
			for j := 0; j < 100; j++ {
				summary.WithLabelValues(method).Observe(float64(j) / 100.0)
			}
		}
	}

	if histogramCount > 0 {
		histogram := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: []float64{100, 500, 1000, 5000, 10000},
			},
			[]string{"method"},
		)
		registry.MustRegister(histogram)

		methods := []string{"GET", "POST", "PUT"}

		for i := 0; i < histogramCount; i++ {
			method := methods[i%len(methods)]
			// Observe some values to populate the histogram
			for j := 0; j < 100; j++ {
				histogram.WithLabelValues(method).Observe(float64(j * 50))
			}
		}
	}
}

func createTargetInfo(registry *prometheus.Registry) {
	targetInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "target_info",
		Help: "Target metadata",
		ConstLabels: prometheus.Labels{
			"service_name":    "benchmark-service",
			"service_version": "1.0.0",
			"environment":     "test",
			"region":          "us-west",
			"cluster":         "prod",
		},
	})
	registry.MustRegister(targetInfo)
	targetInfo.Set(1)
}

func createScopeInfo(registry *prometheus.Registry) {
	scopeInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "otel_scope_info",
		Help: "Scope metadata",
		ConstLabels: prometheus.Labels{
			"otel_scope_name":    "benchmark.scope",
			"otel_scope_version": "1.0.0",
		},
	})
	registry.MustRegister(scopeInfo)
	scopeInfo.Set(1)
}

// encodeMetrics encodes the metrics from the registry to the specified format
func encodeMetrics(registry *prometheus.Registry, proto protocol) string {
	metricFamilies, err := registry.Gather()
	if err != nil {
		panic(err) // In benchmark, we want to fail fast if metrics can't be gathered
	}

	var contentType expfmt.Format
	switch proto {
	case protocolOpenMetrics:
		contentType = expfmt.NewFormat(expfmt.TypeOpenMetrics)
	case protocolPromProtobuf:
		contentType = expfmt.NewFormat(expfmt.TypeProtoDelim)
	default: // protocolPromText
		contentType = expfmt.NewFormat(expfmt.TypeTextPlain)
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, contentType)

	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			panic(err)
		}
	}

	return buf.String()
}

// createMockPrometheusServer creates a mock HTTP server that serves metric data
func createMockPrometheusServer(metricData string, proto protocol) *httptest.Server {
	// For benchmarking, we need a server that can handle unlimited requests
	// without the waitgroup Done() pattern used in tests
	return httptest.NewServer(newBenchmarkHandler(metricData, proto))
}

// benchmarkHandler is a simple HTTP handler for benchmark mock servers
type benchmarkHandler struct {
	data           string
	useOpenMetrics bool
	useProtoBuf    bool
}

func newBenchmarkHandler(data string, proto protocol) *benchmarkHandler {
	return &benchmarkHandler{
		data:           data,
		useOpenMetrics: proto == protocolOpenMetrics,
		useProtoBuf:    proto == protocolPromProtobuf,
	}
}

func (h *benchmarkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/metrics" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Set appropriate content type based on protocol
	switch {
	case h.useProtoBuf:
		w.Header().Set("Content-Type", "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited")
	case h.useOpenMetrics:
		w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(h.data))
}

// waitForScrape waits for at least one scrape to complete
func waitForScrape(b *testing.B, consumer *consumertest.MetricsSink, timeout time.Duration) {
	b.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(consumer.AllMetrics()) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.Fatal("Timeout waiting for scrape to complete")
}

// waitForNextScrape waits for a specific number of scrapes to be collected
func waitForNextScrape(b *testing.B, consumer *consumertest.MetricsSink, targetCount int, timeout time.Duration) {
	b.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(consumer.AllMetrics()) >= targetCount {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.Fatalf("Timeout waiting for scrape %d to complete (got %d)", targetCount, len(consumer.AllMetrics()))
}
