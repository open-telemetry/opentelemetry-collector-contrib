# OTEL Histograms Metrics Generator

A command-line tool that generates and exports OpenTelemetry histogram metrics using predefined test cases. This generator is designed for testing histogram processing in OpenTelemetry collectors and exporters.

## Overview

This tool continuously generates histogram metrics with various data patterns and exports them via OTLP HTTP. It includes both valid and invalid histogram test cases to verify proper handling of edge cases and error conditions.

## Features

- **Continuous Metric Generation**: Exports metrics every 10 seconds
- **Comprehensive Test Cases**: 25+ valid histogram scenarios covering various data patterns
- **Invalid Test Cases**: 12 invalid histogram scenarios for error handling validation
- **OTLP HTTP Export**: Configurable endpoint with insecure connections supported
- **Realistic Data**: Test cases include real-world scenarios like payment services, API gateways, and batch processors

## Usage

### Basic Usage

```bash
go run main.go
```

The generator will start exporting metrics to `localhost:4318` (default OTLP HTTP endpoint) every 10 seconds.

### Configuration

The tool is currently configured with hardcoded settings but can be modified in `main.go`:

```go
exporter, err := createExporter(&Config{
    UseHTTP:  true,
    Insecure: true,
})
```

## Test Cases

### Valid Histogram Scenarios

The generator includes diverse histogram patterns:

- **Basic Histogram**: Standard distribution with 101 samples
- **Tail Heavy**: Distribution with heavy concentration in upper buckets
- **Single/Two Buckets**: Minimal bucket configurations
- **Large Numbers**: High-value measurements (up to 1B)
- **Small Numbers**: Micro-precision measurements
- **Many Buckets**: Up to 325 buckets for testing scalability
- **Negative Values**: Histograms with negative boundaries
- **Missing Min/Max**: Various combinations of undefined extrema
- **Unbounded**: Histograms without explicit boundaries

### Invalid Test Cases

Error scenarios for validation:

- Non-ascending boundaries
- Count/bucket mismatches
- Invalid min/max relationships
- NaN/Inf values
- Inconsistent sum calculations

## Metric Structure

Each generated metric includes:

```go
type HistogramInput struct {
    Count      uint64              // Total sample count
    Sum        float64             // Sum of all samples
    Min        *float64            // Minimum value (optional)
    Max        *float64            // Maximum value (optional)
    Boundaries []float64           // Bucket boundaries
    Counts     []uint64            // Counts per bucket
    Attributes map[string]string   // Metric attributes
}
```

## Export Format

Metrics are exported as OTLP histogram data points with:
- Delta temporality
- Configurable attributes (service.name, etc.)
- Proper bucket counts and boundaries
- Min/max extrema when available

## Development

### Adding New Test Cases

1. Add new cases to `TestCases()` or `InvalidTestCases()` in the histograms package
2. Define the input histogram structure
3. Specify expected metrics for validation

### Modifying Export Configuration

Update the `Config` struct in `main.go` to customize:
- Endpoint URLs
- Security settings
- Headers
- Export intervals

## Use Cases

- **Testing Histogram Processing**: Validate collector histogram handling
- **Performance Testing**: Generate high-volume histogram data
- **Edge Case Validation**: Test error handling with invalid histograms
- **Integration Testing**: End-to-end OTLP histogram pipeline testing