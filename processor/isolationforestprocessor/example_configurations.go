// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// example_configurations.go - Real-world configuration examples and usage patterns
package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"fmt"
)

// This file contains example configurations and usage patterns for the isolation forest processor.
// These examples demonstrate how to configure the processor for different scenarios and use cases.

// ExampleBasicConfiguration shows a simple setup for general anomaly detection
func ExampleBasicConfiguration() {
	configYAML := `
processors:
  isolationforest:
    # Basic anomaly detection for all telemetry types
    forest_size: 100
    subsample_size: 256
    contamination_rate: 0.1
    
    mode: "enrich"  # Add anomaly scores without filtering
    threshold: 0.7  # Conservative threshold
    
    # Model updates
    training_window: "24h"    # Use 24 hours of data for training
    update_frequency: "1h"    # Retrain every hour
    min_samples: 1000         # Need 1000 samples before training
    
    # Feature extraction for different signal types
    features:
      traces: ["duration", "error", "http.status_code"]
      metrics: ["value", "rate_of_change"]
      logs: ["severity_number", "timestamp_gap"]

exporters:
  jaeger:
    endpoint: jaeger:14250
  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [isolationforest]
      exporters: [jaeger]
    metrics:
      receivers: [otlp]
      processors: [isolationforest]
      exporters: [prometheus]
`
	fmt.Println("Basic Configuration:")
	fmt.Println(configYAML)
}

// ExampleCostOptimizationConfiguration shows how to use filtering to reduce storage costs
func ExampleCostOptimizationConfiguration() {
	configYAML := `
processors:
  isolationforest:
    mode: "filter"          # Filter out normal data
    threshold: 0.6          # Lower threshold to keep more potential issues
    forest_size: 150        # Larger forest for better accuracy
    
    # More aggressive updates for faster adaptation
    update_frequency: "30m"
    training_window: "12h"
    
    features:
      traces: ["duration", "error", "http.status_code", "service.name"]
      
  # Additional sampling for normal data
  probabilistic_sampler:
    sampling_percentage: 5  # Keep only 5% of normal data
    
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  # Send all data to analysis backend
  otlp/analysis:
    endpoint: analysis-backend:4317
  
  # Send only anomalies to alerting system  
  otlp/alerts:
    endpoint: alerting-system:4317

service:
  pipelines:
    traces/analysis:
      receivers: [otlp]
      processors: [isolationforest, probabilistic_sampler, batch]
      exporters: [otlp/analysis]
      
    traces/alerts:
      receivers: [otlp]
      processors: [isolationforest, batch]
      exporters: [otlp/alerts]
`
	fmt.Println("Cost Optimization Configuration:")
	fmt.Println(configYAML)
}

// ExampleMultiEnvironmentConfiguration demonstrates environment-specific models
func ExampleMultiEnvironmentConfiguration() {
	configYAML := `
processors:
  isolationforest:
    # Multiple models for different environments and services
    models:
      - name: "production_web"
        selector:
          deployment.environment: "prod"
          service.name: "web-frontend"
        threshold: 0.9        # Very strict for production web service
        forest_size: 200      # Large forest for high accuracy
        features: ["duration", "error", "http.status_code", "http.method"]
        contamination_rate: 0.02  # Expect very few anomalies in prod
        
      - name: "production_api"
        selector:
          deployment.environment: "prod"
          service.type: "api"
        threshold: 0.85
        forest_size: 150
        features: ["duration", "error", "http.status_code", "db.duration"]
        contamination_rate: 0.05
        
      - name: "staging"
        selector:
          deployment.environment: "staging"
        threshold: 0.6        # More permissive for staging
        forest_size: 50       # Smaller forest, less critical
        features: ["duration", "error"]
        contamination_rate: 0.2
        
      - name: "development"
        selector:
          deployment.environment: "dev"
        threshold: 0.5        # Very permissive for dev
        forest_size: 20
        features: ["duration", "error"]
        contamination_rate: 0.3
    
    # Global settings
    mode: "both"            # Enrich and filter
    training_window: "6h"   # Shorter window for faster adaptation
    update_frequency: "15m" # Frequent updates
    
    # Performance settings for high-volume production
    performance:
      max_memory_mb: 1024
      batch_size: 2000
      parallel_workers: 8

processors:
  # Route different anomaly severities to different exporters
  routing:
    table:
      - statement: route() where attributes["anomaly.isolation_score"] > 0.9
        pipelines: [traces/critical]
      - statement: route() where attributes["anomaly.isolation_score"] > 0.7  
        pipelines: [traces/warning]
      - statement: route()
        pipelines: [traces/normal]

exporters:
  otlp/critical:
    endpoint: pagerduty-integration:4317
  otlp/warning:
    endpoint: slack-integration:4317
  otlp/normal:
    endpoint: storage-backend:4317

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [isolationforest, routing]
      
    traces/critical:
      receivers: [routing]
      processors: [batch]
      exporters: [otlp/critical]
      
    traces/warning:
      receivers: [routing]  
      processors: [batch]
      exporters: [otlp/warning]
      
    traces/normal:
      receivers: [routing]
      processors: [batch]
      exporters: [otlp/normal]
`
	fmt.Println("Multi-Environment Configuration:")
	fmt.Println(configYAML)
}

// ExampleHighPerformanceConfiguration shows optimization for high-throughput scenarios
func ExampleHighPerformanceConfiguration() {
	configYAML := `
processors:
  isolationforest:
    # Optimized for high throughput
    forest_size: 50         # Smaller forest for speed
    subsample_size: 128     # Smaller samples for speed
    threshold: 0.8          # Higher threshold to reduce false positives
    
    mode: "enrich"          # Only enrich, don't filter (faster)
    
    # Reduced update frequency for performance
    training_window: "2h"   # Shorter window
    update_frequency: "5m"  # Less frequent updates
    min_samples: 500        # Lower minimum for faster startup
    
    # Minimal feature set for speed
    features:
      traces: ["duration", "error"]  # Only essential features
      metrics: ["value"]
      logs: ["severity_number"]
    
    # Performance tuning
    performance:
      max_memory_mb: 512
      batch_size: 5000      # Large batches for efficiency
      parallel_workers: 16  # Many workers for parallel processing
      
  # Additional processing optimizations
  batch:
    timeout: 200ms          # Quick batching
    send_batch_size: 8192   # Large batches
    
  memory_limiter:
    limit_mib: 2048         # Prevent memory exhaustion

exporters:
  otlp/fast:
    endpoint: fast-backend:4317
    sending_queue:
      queue_size: 10000     # Large queue for bursts
    retry_on_failure:
      enabled: false        # Disable retries for speed

service:
  extensions: [memory_ballast]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, isolationforest, batch]
      exporters: [otlp/fast]
`
	fmt.Println("High Performance Configuration:")
	fmt.Println(configYAML)
}

// ExampleDebuggingConfiguration shows settings optimized for troubleshooting
func ExampleDebuggingConfiguration() {
	configYAML := `
processors:
  isolationforest:
    # Debug-friendly settings
    forest_size: 20         # Small forest for faster debugging
    threshold: 0.3          # Low threshold to catch more anomalies
    mode: "both"            # Both enrich and filter for testing
    
    # Rapid updates for quick iteration
    training_window: "10m"
    update_frequency: "1m"
    min_samples: 50
    
    # Comprehensive features for analysis
    features:
      traces: ["duration", "error", "http.status_code", "service.name", "operation.name"]
      metrics: ["value", "rate_of_change", "timestamp_gap"]  
      logs: ["severity_number", "timestamp_gap", "message_length"]
    
    # Debug output attributes
    score_attribute: "debug.anomaly_score"
    classification_attribute: "debug.is_anomaly"
    
    performance:
      max_memory_mb: 128    # Limited memory for local testing
      batch_size: 100
      parallel_workers: 2

  # Add debug attributes
  attributes:
    actions:
      - key: debug.processor_version
        value: "1.0.0-debug"
        action: insert
      - key: debug.timestamp  
        value: "%{time_unix}"
        action: insert

exporters:
  debug:
    verbosity: detailed     # Verbose debug output
  
  file:
    path: ./debug_traces.json
    format: json

service:
  telemetry:
    logs:
      level: debug          # Enable debug logging
    metrics:
      level: detailed
      
  pipelines:
    traces:
      receivers: [otlp]
      processors: [isolationforest, attributes]
      exporters: [debug, file]
`
	fmt.Println("Debugging Configuration:")
	fmt.Println(configYAML)
}

// ExampleAdaptiveWindowConfiguration shows adaptive window sizing for dynamic workloads
func ExampleAdaptiveWindowConfiguration() {
	configYAML := `
processors:
  isolationforest:
    # Dynamic window sizing for varying traffic patterns
    forest_size: 100
    mode: "enrich"
    threshold: 0.8
    
    # Adaptive window configuration
    adaptive_window:
      enabled: true
      min_window_size: 1000
      max_window_size: 50000
      memory_limit_mb: 256
      adaptation_rate: 0.1
      velocity_threshold: 50.0
      stability_check_interval: 5m
    
    # Standard settings
    training_window: "12h"
    update_frequency: "30m"
    
    features:
      traces: ["duration", "error"]
      metrics: ["value", "rate_of_change"]
    
    performance:
      max_memory_mb: 512
      batch_size: 1000


exporters:
  prometheus:
    endpoint: "0.0.0.0:8888"


service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [isolationforest]
      exporters: [prometheus]
`
	fmt.Println("Adaptive Window Configuration:")
	fmt.Println(configYAML)
}
