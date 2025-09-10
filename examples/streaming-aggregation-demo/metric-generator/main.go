package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Get collector endpoints from environment variables
	rawEndpoint := os.Getenv("RAW_COLLECTOR_ENDPOINT")
	if rawEndpoint == "" {
		rawEndpoint = "localhost:4317"
	}
	
	aggregatedEndpoint := os.Getenv("AGGREGATED_COLLECTOR_ENDPOINT")
	if aggregatedEndpoint == "" {
		aggregatedEndpoint = "localhost:4318"
	}

	log.Printf("Connecting to collectors - Raw: %s, Aggregated: %s", rawEndpoint, aggregatedEndpoint)

	// Initialize two metric providers - one for raw, one for aggregated
	rawProvider, err := initMetricProvider(ctx, rawEndpoint, "raw-metrics")
	if err != nil {
		log.Fatalf("Failed to initialize raw metric provider: %v", err)
	}
	defer rawProvider.Shutdown(ctx)

	aggregatedProvider, err := initMetricProvider(ctx, aggregatedEndpoint, "aggregated-metrics")
	if err != nil {
		log.Fatalf("Failed to initialize aggregated metric provider: %v", err)
	}
	defer aggregatedProvider.Shutdown(ctx)

	// Create meters for both pipelines
	rawMeter := rawProvider.Meter("demo.raw")
	aggMeter := aggregatedProvider.Meter("demo.aggregated")

	// Create instruments for different metric types
	if err := createAndEmitMetrics(ctx, rawMeter, aggMeter, rawProvider, aggregatedProvider); err != nil {
		log.Fatalf("Failed to emit metrics: %v", err)
	}
}

func initMetricProvider(ctx context.Context, endpoint string, serviceName string) (*sdkmetric.MeterProvider, error) {
	// Create OTLP exporter with CUMULATIVE temporality for counters
	// This is the default and most natural temporality for counters
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
		// Remove the temporality selector - use defaults (cumulative for counters)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", "demo"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create meter provider with 1 second export interval
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(1*time.Second),
			),
		),
	)

	return provider, nil
}

func createAndEmitMetrics(ctx context.Context, rawMeter, aggMeter metric.Meter, rawProvider, aggregatedProvider *sdkmetric.MeterProvider) error {
	// Create gauge for temperature
	rawTempGauge, err := rawMeter.Float64ObservableGauge(
		"temperature_celsius",
		metric.WithDescription("Current temperature in Celsius"),
		metric.WithUnit("Cel"),
	)
	if err != nil {
		return err
	}

	aggTempGauge, err := aggMeter.Float64ObservableGauge(
		"temperature_celsius",
		metric.WithDescription("Current temperature in Celsius"),
		metric.WithUnit("Cel"),
	)
	if err != nil {
		return err
	}

	// Create counter for requests
	rawRequestCounter, err := rawMeter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	aggRequestCounter, err := aggMeter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Create histogram for response times
	rawResponseHist, err := rawMeter.Float64Histogram(
		"http_response_time_ms",
		metric.WithDescription("HTTP response time in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	aggResponseHist, err := aggMeter.Float64Histogram(
		"http_response_time_ms",
		metric.WithDescription("HTTP response time in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	// Create up-down counter for active connections
	rawActiveConnections, err := rawMeter.Int64UpDownCounter(
		"active_connections",
		metric.WithDescription("Number of active connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	aggActiveConnections, err := aggMeter.Int64UpDownCounter(
		"active_connections",
		metric.WithDescription("Number of active connections"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}

	// Register callbacks for observable gauges
	temperature := 20.0
	_, err = rawMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			// Simulate temperature fluctuation with bounds
			// Add random change between -1 and +1 degree
			temperature += (rand.Float64() - 0.5) * 2
			
			// Keep temperature within realistic bounds (15°C to 30°C)
			// This represents typical server room temperature range
			if temperature < 15.0 {
				temperature = 15.0
			} else if temperature > 30.0 {
				temperature = 30.0
			}
			
			o.ObserveFloat64(rawTempGauge, temperature,
				metric.WithAttributes(
					attribute.String("location", "server_room"),
					attribute.String("sensor", "sensor_1"),
				),
			)
			return nil
		},
		rawTempGauge,
	)
	if err != nil {
		return err
	}

	_, err = aggMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			o.ObserveFloat64(aggTempGauge, temperature,
				metric.WithAttributes(
					attribute.String("location", "server_room"),
					attribute.String("sensor", "sensor_1"),
				),
			)
			return nil
		},
		aggTempGauge,
	)
	if err != nil {
		return err
	}

	// Start emitting metrics
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	totalRequests := int64(0)
	activeConns := int64(100)  // Start with a higher baseline
	
	// Initialize the UpDownCounter with the starting value
	rawActiveConnections.Add(ctx, activeConns)
	aggActiveConnections.Add(ctx, activeConns)

	log.Println("Starting metric generation... Press Ctrl+C to stop")
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down metric generator")
			return nil
		case <-ticker.C:
			// Emit counter metrics (simulate 5-15 requests per second)
			requests := rand.Int63n(11) + 5
			totalRequests += requests
			
			// Emit to both raw and aggregated WITHOUT any attributes
			// This ensures we're always updating the same time series
			rawRequestCounter.Add(ctx, requests)
			aggRequestCounter.Add(ctx, requests)

			// Emit histogram metrics (simulate response times)
			for i := int64(0); i < requests; i++ {
				responseTime := generateResponseTime()
				rawResponseHist.Record(ctx, responseTime,
					metric.WithAttributes(
						attribute.String("endpoint", randomEndpoint()),
					),
				)
				aggResponseHist.Record(ctx, responseTime,
					metric.WithAttributes(
						attribute.String("endpoint", randomEndpoint()),
					),
				)
			}

			// Update active connections (simulate connection changes)
			// Keep connections between 50 and 150
			connChange := rand.Int63n(21) - 10 // -10 to +10
			newActiveConns := activeConns + connChange
			
			// Ensure we stay within reasonable bounds (50-150)
			if newActiveConns < 50 {
				newActiveConns = 50
				connChange = newActiveConns - activeConns
			} else if newActiveConns > 150 {
				newActiveConns = 150
				connChange = newActiveConns - activeConns
			}
			
			activeConns = newActiveConns
			
			// Add the change to the up-down counter
			if connChange != 0 {
				rawActiveConnections.Add(ctx, connChange)
				aggActiveConnections.Add(ctx, connChange)
			}

			log.Printf("Emitted metrics - Requests: %d (Total: %d), Active Connections: %d, Temp: %.2f°C",
				requests, totalRequests, activeConns, temperature)
			
			// Force a flush to ensure metrics are sent immediately
			// This helps with debugging to ensure metrics are actually being sent
			rawProvider.ForceFlush(ctx)
			aggregatedProvider.ForceFlush(ctx)
		}
	}
}

func randomMethod() string {
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	return methods[rand.Intn(len(methods))]
}

func randomStatus() string {
	statuses := []string{"200", "201", "400", "404", "500"}
	weights := []int{70, 10, 5, 10, 5} // Weighted distribution
	
	total := 0
	for _, w := range weights {
		total += w
	}
	
	r := rand.Intn(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return statuses[i]
		}
	}
	return statuses[0]
}

func randomEndpoint() string {
	endpoints := []string{"/api/users", "/api/products", "/api/orders", "/health", "/metrics"}
	return endpoints[rand.Intn(len(endpoints))]
}

func generateResponseTime() float64 {
	// Generate response times with realistic distribution
	// Most requests are fast (20-100ms), some are medium (100-500ms), few are slow (500-2000ms)
	r := rand.Float64()
	if r < 0.7 { // 70% fast
		return 20 + rand.Float64()*80
	} else if r < 0.95 { // 25% medium
		return 100 + rand.Float64()*400
	} else { // 5% slow
		return 500 + rand.Float64()*1500
	}
}
