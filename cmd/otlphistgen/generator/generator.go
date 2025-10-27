// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package generator provides histogram generation capabilities for OpenTelemetry metrics testing.
// This package is designed to generate realistic histogram data using various statistical distributions
// and can optionally publish the generated metrics to OTLP endpoints.
//
// The package is organized into several focused modules:
// - types.go: Core data structures and types
// - histogram_generator.go: Main histogram generation logic
// - distributions.go: Statistical distribution functions
// - otlp_publisher.go: OTLP endpoint publishing functionality
//
// Example usage:
//
//	generator := NewHistogramGenerator(GenerationOptions{
//		Seed:     12345,
//		Endpoint: "localhost:4318",
//	})
//
//	result, err := generator.GenerateAndPublishHistograms(
//		HistogramInput{
//			Count:      1000,
//			Min:        ptr(10.0),
//			Max:        ptr(200.0),
//			Boundaries: []float64{25, 50, 75, 100, 150},
//			Attributes: map[string]string{"service.name": "test-service"},
//		},
//		func(rnd *rand.Rand, t time.Time) float64 {
//			return NormalRandom(rnd, 50, 15)
//		},
//	)
package generator
