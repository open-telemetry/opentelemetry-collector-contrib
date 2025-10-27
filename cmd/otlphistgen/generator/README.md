# Histogram Generator

A Go package for generating realistic histogram data for OpenTelemetry metrics testing. This package provides statistical distribution functions and can optionally publish generated metrics to OTLP endpoints.

## Architecture

The package is organized into focused modules for maintainability and scalability:

```
cmd/generator/
├── generator.go          # Package documentation and overview
├── types.go             # Core data structures and types
├── histogram_generator.go # Main histogram generation logic
├── distributions.go     # Statistical distribution functions
├── otlp_publisher.go    # OTLP endpoint publishing functionality
└── example_test.go      # Usage examples
```

### Key Components

- **HistogramGenerator**: Main generator that creates histogram data from statistical distributions
- **OTLPPublisher**: Handles publishing metrics to OpenTelemetry Protocol endpoints using both telemetrygen and custom OTLP
- **Distribution Functions**: Various statistical distributions (Normal, Exponential, Gamma, etc.)
- **Types**: Shared data structures for histogram inputs, outputs, and configuration

### Publishing Approach

The package uses the official OpenTelemetry telemetrygen package for all metric publishing, ensuring:

- **Standard Compliance**: Full compatibility with OTLP endpoints
- **Reliability**: Uses the same code as the official telemetrygen tool
- **Simplicity**: Clean API with `metrics.Start(cfg)` under the hood
- **Flexibility**: Supports Gauge, Sum, and Histogram metric types

The histogram generator creates realistic data distributions, while telemetrygen handles the actual OTLP publishing.

## Usage

### Basic Generation

```go
generator := NewHistogramGenerator(GenerationOptions{
    Seed: 12345, // For reproducible results
})

input := HistogramInput{
    Count:      1000,
    Min:        ptr(10.0),
    Max:        ptr(200.0),
    Boundaries: []float64{25, 50, 75, 100, 150},
    Attributes: map[string]string{"service.name": "test-service"},
}

result, err := generator.GenerateHistogram(input, func(rnd *rand.Rand, t time.Time) float64 {
    return NormalRandom(rnd, 75, 25) // mean=75, stddev=25
})
```

### Generation with Publishing

```go
generator := NewHistogramGenerator(GenerationOptions{
    Seed:     time.Now().UnixNano(),
    Endpoint: "localhost:4318", // OTLP HTTP endpoint
})

result, err := generator.GenerateAndPublishHistograms(input, valueFunc)
```

### Direct Publishing with Telemetrygen

You can also use telemetrygen directly for different metric types:

```go
publisher := NewOTLPPublisher("localhost:4318")

// Send different types of metrics using telemetrygen
err := publisher.SendSumMetric("requests_total", 100)
err = publisher.SendGaugeMetric("cpu_usage", 75.5)
err = publisher.SendHistogramMetricSimple("response_time")
```

This approach uses the official OpenTelemetry telemetrygen package under the hood, ensuring compatibility with standard OTLP endpoints.

## Available Distributions

- **NormalRandom**: Normal (Gaussian) distribution
- **ExponentialRandom**: Exponential distribution
- **GammaRandom**: Gamma distribution
- **LogNormalRandom**: Log-normal distribution
- **WeibullRandom**: Weibull distribution
- **BetaRandom**: Beta distribution

### Time-based Functions

- **SinusoidalValue**: Sinusoidal patterns with noise
- **SpikyValue**: Baseline with occasional spikes
- **TrendingValue**: Linear trend with noise

## Design Principles

### Single Responsibility
Each file has a focused purpose:
- `types.go`: Data structures only
- `distributions.go`: Statistical functions only
- `histogram_generator.go`: Core generation logic only
- `otlp_publisher.go`: Publishing logic only

### Dependency Injection
The generator accepts value functions, allowing for flexible distribution selection and custom patterns.

### Testability
All components are designed for easy unit testing with deterministic seeds and dependency injection.

### Extensibility
New distributions can be added to `distributions.go` without affecting other components.

## Features

- ✅ **Statistical Distributions**: Multiple distribution functions for realistic data
- ✅ **OTLP Publishing**: Direct integration with OpenTelemetry Protocol endpoints
- ✅ **Flexible Generation**: Custom value functions and deterministic seeds
- ✅ **Multiple Metric Types**: Support for Gauge, Sum, and Histogram metrics

## Future Enhancements

1. **Additional Publishers**: Support for Prometheus, StatsD, etc.
2. **More Distributions**: Poisson, Binomial, etc.
3. **Validation**: Input validation for histogram consistency
4. **Batch Generation**: Generate multiple histograms efficiently
5. **Configuration Files**: YAML/JSON configuration support

## Testing

The package includes comprehensive examples in `example_test.go` and integrates with the test cases in `share/testdata/histograms/`.

Run tests:
```bash
go test ./cmd/generator/...
```

## Integration

This generator is used by:
- `share/testdata/histograms/histograms.go`: Test case generation
- Various metric exporters for testing realistic data patterns
- Performance testing tools for load generation