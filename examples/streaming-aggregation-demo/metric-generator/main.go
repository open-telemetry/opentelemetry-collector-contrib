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

	// Create meter provider with 5 second export interval
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(5*time.Second),
			),
		),
		// Configure exponential histogram ONLY for request_duration_seconds
		// This keeps http_response_time_ms as a regular histogram
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: "request_duration_seconds",
				Kind: sdkmetric.InstrumentKindHistogram,
			},
			sdkmetric.Stream{
				Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
					MaxSize:  160, // Maximum number of buckets
					MaxScale: 20,  // Maximum scale factor
				},
			},
		)),
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

	// Create exponential histogram for request duration (following native-histogram-otel pattern)
	// Note: raw_ prefix will be added by the raw collector's metricstransform processor
	rawRequestDuration, err := rawMeter.Float64Histogram(
		"request_duration_seconds",
		metric.WithDescription("Duration of requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	aggRequestDuration, err := aggMeter.Float64Histogram(
		"request_duration_seconds",
		metric.WithDescription("Duration of requests in seconds"),
		metric.WithUnit("s"),
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
	temperature := map[string]float64{}

	// Initialize temperature for each combination
	datacenters := []string{"dc-1", "dc-2"}
	locations := []string{"server-room-1", "server-room-2", "server-room-3"}
	sensors := []string{"sensor-1", "sensor-2", "sensor-3", "sensor-4"}

	for _, dc := range datacenters {
		for _, loc := range locations {
			for _, sensor := range sensors {
				key := fmt.Sprintf("%s-%s-%s", dc, loc, sensor)
				temperature[key] = 20.0 + rand.Float64()*10 // Start with 20-30°C range
			}
		}
	}

	_, err = rawMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			// Generate temperature readings for each datacenter/location/sensor combination
			for _, dc := range datacenters {
				for _, loc := range locations {
					for _, sensor := range sensors {
						key := fmt.Sprintf("%s-%s-%s", dc, loc, sensor)

						// Simulate temperature fluctuation with bounds
						// Add random change between -1 and +1 degree
						temperature[key] += (rand.Float64() - 0.5) * 2

						// Keep temperature within realistic bounds (15°C to 35°C)
						// Different ranges for different datacenters/locations
						minTemp := 15.0
						maxTemp := 35.0

						// dc-1 tends to run cooler
						if dc == "dc-1" {
							maxTemp = 30.0
						}
						// dc-2 tends to run warmer
						if dc == "dc-2" {
							minTemp = 18.0
						}

						if temperature[key] < minTemp {
							temperature[key] = minTemp
						} else if temperature[key] > maxTemp {
							temperature[key] = maxTemp
						}

						o.ObserveFloat64(rawTempGauge, temperature[key],
							metric.WithAttributes(
								attribute.String("datacenter", dc),
								attribute.String("location", loc),
								attribute.String("sensor", sensor),
							),
						)
					}
				}
			}
			return nil
		},
		rawTempGauge,
	)
	if err != nil {
		return err
	}

	_, err = aggMeter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			// Generate the same temperature readings for aggregated metrics
			for _, dc := range datacenters {
				for _, loc := range locations {
					for _, sensor := range sensors {
						key := fmt.Sprintf("%s-%s-%s", dc, loc, sensor)

						o.ObserveFloat64(aggTempGauge, temperature[key],
							metric.WithAttributes(
								attribute.String("datacenter", dc),
								attribute.String("location", loc),
								attribute.String("sensor", sensor),
							),
						)
					}
				}
			}
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

	// Track active connections for each service/region/protocol/instance combination
	// This ensures we never go below zero
	type connKey struct {
		service, region, protocol, instance string
	}
	activeConnections := make(map[connKey]int64)

	services := []string{"web", "api", "database"}
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	protocols := []string{"http", "grpc", "tcp"}
	instances := []string{"srv-01", "srv-02", "srv-03"}

	// Initialize with baseline connections (10-30 per combination)
	for _, service := range services {
		for _, region := range regions {
			for _, protocol := range protocols {
				for _, instance := range instances {
					initialConns := rand.Int63n(21) + 10 // 10-30 connections
					key := connKey{service, region, protocol, instance}
					activeConnections[key] = initialConns

					rawActiveConnections.Add(ctx, initialConns,
						metric.WithAttributes(
							attribute.String("service", service),
							attribute.String("region", region),
							attribute.String("protocol", protocol),
							attribute.String("instance", instance),
						),
					)
					aggActiveConnections.Add(ctx, initialConns,
						metric.WithAttributes(
							attribute.String("service", service),
							attribute.String("region", region),
							attribute.String("protocol", protocol),
							attribute.String("instance", instance),
						),
					)
				}
			}
		}
	}

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

			// Generate HTTP requests with essential labels for testing aggregation
			for i := int64(0); i < requests; i++ {
				method := randomMethod()
				status := randomStatus()
				endpoint := randomEndpoint()
				instance := randomInstance()

				rawRequestCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("method", method),
						attribute.String("status_code", status),
						attribute.String("endpoint", endpoint),
						attribute.String("instance", instance),
					),
				)
				aggRequestCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("method", method),
						attribute.String("status_code", status),
						attribute.String("endpoint", endpoint),
						attribute.String("instance", instance),
					),
				)
			}

			// Emit histogram metrics (simulate response times)
			for i := int64(0); i < requests; i++ {
				responseTime := generateResponseTime()
				endpoint := randomEndpoint()
				instance := randomInstance()

				rawResponseHist.Record(ctx, responseTime,
					metric.WithAttributes(
						attribute.String("endpoint", endpoint),
						attribute.String("instance", instance),
					),
				)
				aggResponseHist.Record(ctx, responseTime,
					metric.WithAttributes(
						attribute.String("endpoint", endpoint),
						attribute.String("instance", instance),
					),
				)
			}

			// Emit exponential histogram metrics with varied endpoints and methods
			for i := int64(0); i < requests; i++ {
				latency := generateLatencySeconds()

				// Generate varied endpoints, methods, and status codes for testing label handling
				endpoints := []string{"/api/data", "/api/users", "/api/orders", "/api/products", "/api/payments"}
				methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
				statusCodes := []string{"200", "201", "400", "404", "500"}

				endpoint := endpoints[i%int64(len(endpoints))]
				method := methods[i%int64(len(methods))]
				// Use random status codes so each endpoint gets multiple status codes
				// This creates realistic cardinality for testing label grouping
				statusCode := statusCodes[rand.Intn(len(statusCodes))]

				rawRequestDuration.Record(ctx, latency,
					metric.WithAttributes(
						attribute.String("endpoint", endpoint),
						attribute.String("method", method),
						attribute.String("status_code", statusCode),
					),
				)
				aggRequestDuration.Record(ctx, latency,
					metric.WithAttributes(
						attribute.String("endpoint", endpoint),
						attribute.String("method", method),
						attribute.String("status_code", statusCode),
					),
				)
			}

			// Update active connections with realistic labels (ensuring never negative)
			for _, service := range services {
				for _, region := range regions {
					for _, protocol := range protocols {
						for _, instance := range instances {
							key := connKey{service, region, protocol, instance}
							currentConns := activeConnections[key]

							// Generate realistic connection change based on current state
							var connChange int64

							// If we have many connections, bias toward reducing them
							if currentConns > 50 {
								connChange = rand.Int63n(7) - 4 // -4 to +2 (bias toward reduction)
							} else if currentConns < 10 {
								// If we have few connections, bias toward increasing them
								connChange = rand.Int63n(7) - 2 // -2 to +4 (bias toward increase)
							} else {
								// Normal range, balanced change
								connChange = rand.Int63n(7) - 3 // -3 to +3
							}

							// Ensure we never go below 0
							newConns := currentConns + connChange
							if newConns < 0 {
								connChange = -currentConns // Only reduce to 0
								newConns = 0
							}

							// Update our tracking
							activeConnections[key] = newConns

							// Skip if no change
							if connChange == 0 {
								continue
							}

							// Add the change to the up-down counter with labels
							rawActiveConnections.Add(ctx, connChange,
								metric.WithAttributes(
									attribute.String("service", service),
									attribute.String("region", region),
									attribute.String("protocol", protocol),
									attribute.String("instance", instance),
								),
							)
							aggActiveConnections.Add(ctx, connChange,
								metric.WithAttributes(
									attribute.String("service", service),
									attribute.String("region", region),
									attribute.String("protocol", protocol),
									attribute.String("instance", instance),
								),
							)
						}
					}
				}
			}

			log.Printf("Emitted metrics - Requests: %d (Total: %d), Connection changes emitted, Temp metrics updated",
				requests, totalRequests)
			
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

func randomInstance() string {
	instances := []string{"web-01", "web-02", "web-03", "api-01", "api-02"}
	return instances[rand.Intn(len(instances))]
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

// generateLatencySeconds generates request latency in seconds using the same distribution
// as native-histogram-otel for consistency
func generateLatencySeconds() float64 {
	// Simulate different latency patterns (matching native-histogram-otel)
	var latency float64
	switch rand.Intn(4) {
	case 0:
		// Fast requests (1-10ms)
		latency = 0.001 + rand.Float64()*0.009
	case 1:
		// Normal requests (10-100ms)
		latency = 0.01 + rand.Float64()*0.09
	case 2:
		// Slow requests (100ms-1s)
		latency = 0.1 + rand.Float64()*0.9
	case 3:
		// Very slow requests (1-5s)
		latency = 1.0 + rand.Float64()*4.0
	}
	return latency
}
