// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	rand "math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHistogramGenerator_GenerateHistogram_Example demonstrates basic histogram generation
// CURRENT CAPABILITY: Statistical histogram data generation with custom distributions
func TestHistogramGenerator_GenerateHistogram_Example(t *testing.T) {
	fmt.Println("=== CURRENT GOAL: Statistical Histogram Data Generation ===")

	// Create a generator with a fixed seed for reproducible results
	gen := NewHistogramGenerator(GenerationOptions{
		Seed: 12345,
	})

	// Define histogram input parameters
	input := HistogramInput{
		Count:      1000,
		Min:        ptr(10.0),
		Max:        ptr(200.0),
		Boundaries: []float64{25, 50, 75, 100, 150},
		Attributes: map[string]string{
			"service.name": "payment-service",
			"environment":  "production",
		},
	}

	// Generate histogram using normal distribution
	result, err := gen.GenerateHistogram(input, func(rnd *rand.Rand, _ time.Time) float64 {
		return NormalRandom(rnd, 75, 25) // mean=75, stddev=25
	})
	require.NoError(t, err)

	fmt.Printf("✅ Generated %d samples with sum=%.2f, avg=%.2f\n",
		result.Expected.Count, result.Expected.Sum, result.Expected.Average)
	fmt.Printf("✅ Min=%.2f, Max=%.2f\n", *result.Expected.Min, *result.Expected.Max)
	fmt.Printf("✅ Bucket distribution: %v\n", result.Input.Counts)

	// This example demonstrates histogram generation with statistical distributions.
	// Output will vary due to randomness, but structure is consistent.
}

// TestHieHistogrenerator_GenerateAndPublishHistograms_Example shows telemetrygen integration
// CURRENT LIMITATION: Only supports basic histogram publishing, not custom bucket data
func TestHistogramGenerator_GenerateAndPublishHistograms_Example(t *testing.T) {
	// Make sure collector is running before removing the skip on this function
	t.Skip()
	fmt.Println("=== CURRENT GOAL: Basic OTLP Publishing via Telemetrygen ===")

	// Create a generator with OTLP endpoint
	gen := NewHistogramGenerator(GenerationOptions{
		Seed:     time.Now().UnixNano(),
		Endpoint: "localhost:4318", // OTLP HTTP endpoint
	})

	input := HistogramInput{
		Count:      500,
		Boundaries: []float64{10, 50, 100, 500, 1000},
		Attributes: map[string]string{
			"service.name":    "web-service",
			"service.version": "1.0.0",
			"environment":     "staging",
		},
	}

	// Generate and publish using exponential distribution
	result, err := gen.GenerateAndPublishHistograms(input, func(rnd *rand.Rand, _ time.Time) float64 {
		return ExponentialRandom(rnd, 0.01) // rate=0.01
	})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Printf("✅ Generated and published histogram with %d samples\n", result.Expected.Count)
	fmt.Printf("⚠️  Note: Uses telemetrygen's built-in histogram generation, not custom buckets\n")
}

// TestOTLPPublisher_SendSumMetric_Example demonstrates current telemetrygen integration
// CURRENT CAPABILITY: Basic Sum, Gauge, Histogram via telemetrygen
// MISSING: Delta/Cumulative distinction, Summary, Exponential Histogram
func TestOTLPPublisher_SendSumMetric_Example(t *testing.T) {
	// Make sure collector is running before removing the skip on this function
	t.Skip()
	fmt.Println("=== CURRENT GOAL: Basic Metric Types via Telemetrygen ===")

	publisher := NewOTLPPublisher("localhost:4318")

	// ✅ WORKING: Basic metric types supported by telemetrygen
	err := publisher.SendSumMetric("requests_total", 100)
	if err != nil {
		fmt.Printf("❌ Error sending sum metric: %v\n", err)
		return
	}
	fmt.Println("✅ Sum metric sent (telemetrygen)")

	err = publisher.SendGaugeMetric("cpu_usage", 75.5)
	if err != nil {
		fmt.Printf("❌ Error sending gauge metric: %v\n", err)
		return
	}
	fmt.Println("✅ Gauge metric sent (telemetrygen)")

	err = publisher.SendHistogramMetricSimple("response_time")
	if err != nil {
		fmt.Printf("❌ Error sending histogram metric: %v\n", err)
		return
	}
	fmt.Println("✅ Basic histogram sent (telemetrygen)")

	fmt.Println("\n⚠️  LIMITATIONS:")
	fmt.Println("   - No delta vs cumulative temporality control")
	fmt.Println("   - No summary metrics")
	fmt.Println("   - No exponential histograms")
	fmt.Println("   - No custom histogram bucket data")
}

// TestAllDistributions_Example demonstrates all available statistical distributions
// CURRENT CAPABILITY: Complete statistical distribution library
func TestAllDistributions_Example(_ *testing.T) {
	fmt.Println("=== CURRENT CAPABILITY: All Statistical Distributions ===")

	gen := NewHistogramGenerator(GenerationOptions{Seed: 42})
	boundaries := []float64{10, 25, 50, 75, 100, 150, 200}
	attributes := map[string]string{"test": "distributions"}

	fmt.Println("\n📊 PROBABILITY DISTRIBUTIONS:")

	// Normal Distribution
	result, _ := gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return NormalRandom(rnd, 75, 20) // mean=75, stddev=20
	})
	fmt.Printf("✅ Normal (μ=75, σ=20): avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)

	// Exponential Distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return ExponentialRandom(rnd, 0.02) // rate=0.02
	})
	fmt.Printf("✅ Exponential (λ=0.02): avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)

	// Log-Normal Distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return LogNormalRandom(rnd, 3.5, 0.8) // mu=3.5, sigma=0.8
	})
	fmt.Printf("✅ Log-Normal (μ=3.5, σ=0.8): avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)

	// Gamma Distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return GammaRandom(rnd, 2.0, 25.0) // shape=2, scale=25
	})
	fmt.Printf("✅ Gamma (α=2, β=25): avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)

	// Weibull Distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return WeibullRandom(rnd, 2.5, 80) // shape=2.5, scale=80
	})
	fmt.Printf("✅ Weibull (k=2.5, λ=80): avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)

	// Beta Distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 1000, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return BetaRandom(rnd, 2, 5) * 200 // scale to 0-200 range
	})
	fmt.Printf("✅ Beta (α=2, β=5) scaled: avg=%.1f, samples=%d\n",
		result.Expected.Average, result.Expected.Count)
}

// TestTimeBasedPatterns_Example demonstrates time-based value generation
// CURRENT CAPABILITY: Dynamic time-based patterns for realistic metrics
func TestTimeBasedPatterns_Example(_ *testing.T) {
	fmt.Println("\n=== CURRENT CAPABILITY: Time-Based Patterns ===")

	gen := NewHistogramGenerator(GenerationOptions{Seed: 123})
	boundaries := []float64{20, 40, 60, 80, 100, 120}
	attributes := map[string]string{"pattern": "time-based"}

	fmt.Println("\n🕐 TIME-BASED FUNCTIONS:")

	// Sinusoidal Pattern (daily cycle)
	result, _ := gen.GenerateHistogram(HistogramInput{
		Count: 500, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, t time.Time) float64 {
		return SinusoidalValue(rnd, t, 30, 86400, 0, 60) // 24-hour cycle
	})
	fmt.Printf("✅ Sinusoidal (24h cycle): avg=%.1f, range=[%.1f-%.1f]\n",
		result.Expected.Average, *result.Expected.Min, *result.Expected.Max)

	// Spiky Pattern (occasional bursts)
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 500, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return SpikyValue(rnd, 50, 100, 0.1) // 10% spike probability
	})
	fmt.Printf("✅ Spiky (10%% spikes): avg=%.1f, range=[%.1f-%.1f]\n",
		result.Expected.Average, *result.Expected.Min, *result.Expected.Max)

	// Trending Pattern (gradual increase)
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count: 500, Boundaries: boundaries, Attributes: attributes,
	}, func(rnd *rand.Rand, t time.Time) float64 {
		return TrendingValue(rnd, t, 40, 0.0001, 8) // slow upward trend
	})
	fmt.Printf("✅ Trending (upward): avg=%.1f, range=[%.1f-%.1f]\n",
		result.Expected.Average, *result.Expected.Min, *result.Expected.Max)
}

// TestAdvancedHistogramFeatures_Example shows advanced histogram generation capabilities
// CURRENT CAPABILITY: Custom buckets, attributes, statistical validation
func TestAdvancedHistogramFeatures_Example(_ *testing.T) {
	fmt.Println("\n=== CURRENT CAPABILITY: Advanced Histogram Features ===")

	gen := NewHistogramGenerator(GenerationOptions{Seed: 999})

	fmt.Println("\n🔧 ADVANCED FEATURES:")

	// Custom bucket boundaries for different use cases
	latencyBoundaries := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}
	sizeBoundaries := []float64{1024, 4096, 16384, 65536, 262144, 1048576, 4194304}

	// Latency histogram with log-normal distribution
	result, _ := gen.GenerateHistogram(HistogramInput{
		Count:      2000,
		Min:        ptr(0.5),
		Max:        ptr(10000.0),
		Boundaries: latencyBoundaries,
		Attributes: map[string]string{
			"service.name": "api-gateway",
			"endpoint":     "/users",
			"method":       "GET",
			"status_code":  "200",
		},
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return LogNormalRandom(rnd, 3.0, 1.2) // realistic latency distribution
	})
	fmt.Printf("✅ Latency histogram: %d samples, avg=%.1fms\n",
		result.Expected.Count, result.Expected.Average)
	fmt.Printf("   Buckets: %v\n", result.Input.Counts)

	// File size histogram with exponential distribution
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count:      1500,
		Boundaries: sizeBoundaries,
		Attributes: map[string]string{
			"file_type":   "image",
			"compression": "jpeg",
			"quality":     "high",
		},
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		return ExponentialRandom(rnd, 0.000001) // file size distribution
	})
	fmt.Printf("✅ File size histogram: %d samples, avg=%.0f bytes\n",
		result.Expected.Count, result.Expected.Average)

	// Multi-modal distribution (combining two normals)
	result, _ = gen.GenerateHistogram(HistogramInput{
		Count:      1000,
		Boundaries: []float64{10, 30, 50, 70, 90, 110, 130, 150},
		Attributes: map[string]string{
			"distribution": "bimodal",
			"use_case":     "response_time",
		},
	}, func(rnd *rand.Rand, _ time.Time) float64 {
		if rnd.Float64() < 0.7 {
			return NormalRandom(rnd, 40, 10) // fast responses (70%)
		}
		return NormalRandom(rnd, 120, 15) // slow responses (30%)
	})
	fmt.Printf("✅ Bimodal distribution: avg=%.1f (fast+slow responses)\n",
		result.Expected.Average)
}

// TestCurrentPublishingCapabilities_Example shows what publishing works today
// CURRENT CAPABILITY: Basic OTLP publishing via telemetrygen
func TestCurrentPublishingCapabilities_Example(t *testing.T) {
	// Make sure collector is running before removing the skip on this function
	t.Skip()
	fmt.Println("\n=== CURRENT CAPABILITY: OTLP Publishing ===")

	publisher := NewOTLPPublisher("localhost:4318")

	fmt.Println("\n📡 WORKING METRIC TYPES:")

	// These work with current telemetrygen integration
	metrics := []struct {
		name       string
		metricType string
		value      float64
		status     string
	}{
		{"http_requests_total", "Sum", 1500, "✅ Counter/Sum"},
		{"cpu_utilization", "Gauge", 67.8, "✅ Gauge"},
		{"response_time_histogram", "Histogram", 0, "✅ Basic Histogram"},
		{"memory_usage_bytes", "Gauge", 2147483648, "✅ Gauge (large values)"},
		{"error_rate", "Gauge", 0.025, "✅ Gauge (fractional)"},
	}

	for _, m := range metrics {
		err := publisher.SendMetric(m.name, m.metricType, m.value)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", m.status, err)
		} else {
			fmt.Printf("%s: %s (%.2f)\n", m.status, m.name, m.value)
		}
	}

	fmt.Println("\n⚠️  TELEMETRYGEN LIMITATIONS:")
	fmt.Println("   - No custom histogram bucket data")
	fmt.Println("   - No temporality control (delta vs cumulative)")
	fmt.Println("   - No summary metrics")
	fmt.Println("   - No exponential histograms")
}

// TestShowCapabilities demonstrates all current capabilities
func TestShowCapabilities(t *testing.T) {
	fmt.Println("\n🎯 RUNNING CAPABILITY DEMONSTRATION")

	// Run all the example functions to show capabilities
	TestHistogramGenerator_GenerateHistogram_Example(t)
	fmt.Println()

	// Skip publishing test that requires OTLP endpoint
	fmt.Println("=== SKIPPING: OTLP Publishing (requires running collector) ===")
	fmt.Println()

	TestAllDistributions_Example(t)
	fmt.Println()

	TestTimeBasedPatterns_Example(t)
	fmt.Println()

	TestAdvancedHistogramFeatures_Example(t)
	fmt.Println()

	TestCurrentPublishingCapabilities_Example(t)
	fmt.Println()
}

// TestHistogramGenerator_GenerateHistogram tests the core histogram generation functionality
func TestHistogramGenerator_GenerateHistogram(t *testing.T) {
	gen := NewHistogramGenerator(GenerationOptions{
		Seed: 12345, // Fixed seed for reproducible tests
	})

	input := HistogramInput{
		Count:      1000,
		Min:        ptr(10.0),
		Max:        ptr(200.0),
		Boundaries: []float64{25, 50, 75, 100, 150},
		Attributes: map[string]string{
			"service.name": "test-service",
		},
	}

	result, err := gen.GenerateHistogram(input, func(rnd *rand.Rand, _ time.Time) float64 {
		return NormalRandom(rnd, 75, 25) // mean=75, stddev=25
	})
	// Test basic functionality
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test sample count
	if result.Expected.Count != 1000 {
		t.Errorf("Expected 1000 samples, got %d", result.Expected.Count)
	}

	// Test min/max constraints
	if result.Expected.Min == nil || *result.Expected.Min < 10.0 {
		t.Errorf("Expected min >= 10.0, got %v", result.Expected.Min)
	}
	if result.Expected.Max == nil || *result.Expected.Max > 200.0 {
		t.Errorf("Expected max <= 200.0, got %v", result.Expected.Max)
	}

	// Test that average is reasonable for normal distribution (mean=75)
	if result.Expected.Average < 60 || result.Expected.Average > 90 {
		t.Errorf("Expected average around 75 (60-90), got %.2f", result.Expected.Average)
	}

	// Test bucket counts
	if len(result.Input.Counts) != len(input.Boundaries)+1 {
		t.Errorf("Expected %d buckets, got %d", len(input.Boundaries)+1, len(result.Input.Counts))
	}

	// Test that sum equals count * average (approximately)
	expectedSum := float64(result.Expected.Count) * result.Expected.Average
	if abs(result.Expected.Sum-expectedSum) > 1.0 {
		t.Errorf("Sum/average mismatch: sum=%.2f, count*avg=%.2f", result.Expected.Sum, expectedSum)
	}

	// Test attributes are preserved
	if result.Input.Attributes["service.name"] != "test-service" {
		t.Errorf("Expected service.name=test-service, got %s", result.Input.Attributes["service.name"])
	}
}

// TestHistogramGenerator_Distributions tests all statistical distributions
func TestHistogramGenerator_Distributions(t *testing.T) {
	gen := NewHistogramGenerator(GenerationOptions{Seed: 42})
	boundaries := []float64{10, 25, 50, 75, 100}
	input := HistogramInput{
		Count: 1000, Boundaries: boundaries,
	}

	distributions := []struct {
		name      string
		valueFunc func(*rand.Rand, time.Time) float64
		minAvg    float64
		maxAvg    float64
	}{
		{
			name: "Normal",
			valueFunc: func(rnd *rand.Rand, _ time.Time) float64 {
				return NormalRandom(rnd, 50, 10)
			},
			minAvg: 40, maxAvg: 60,
		},
		{
			name: "Exponential",
			valueFunc: func(rnd *rand.Rand, _ time.Time) float64 {
				return ExponentialRandom(rnd, 0.02)
			},
			minAvg: 30, maxAvg: 70,
		},
		{
			name: "Gamma",
			valueFunc: func(rnd *rand.Rand, _ time.Time) float64 {
				return GammaRandom(rnd, 2.0, 25.0)
			},
			minAvg: 40, maxAvg: 60,
		},
	}

	for _, dist := range distributions {
		t.Run(dist.name, func(t *testing.T) {
			result, err := gen.GenerateHistogram(input, dist.valueFunc)
			if err != nil {
				t.Fatalf("Distribution %s failed: %v", dist.name, err)
			}

			if result.Expected.Count != 1000 {
				t.Errorf("Distribution %s: expected 1000 samples, got %d", dist.name, result.Expected.Count)
			}

			if result.Expected.Average < dist.minAvg || result.Expected.Average > dist.maxAvg {
				t.Errorf("Distribution %s: average %.2f outside expected range [%.1f-%.1f]",
					dist.name, result.Expected.Average, dist.minAvg, dist.maxAvg)
			}
		})
	}
}

// TestOTLPPublisher_Creation tests OTLP publisher creation
func TestOTLPPublisher_Creation(t *testing.T) {
	t.Skip()
	publisher := NewOTLPPublisher("localhost:4318")

	if publisher == nil {
		t.Fatal("Expected publisher to be created, got nil")
	}

	// Test that publisher has the expected endpoint (this would require exposing the field or adding a getter)
	// For now, just test that it was created successfully
}

// TestGenerationOptions tests the generation options
func TestGenerationOptions(t *testing.T) {
	// Test with seed
	gen1 := NewHistogramGenerator(GenerationOptions{Seed: 123})
	gen2 := NewHistogramGenerator(GenerationOptions{Seed: 123})

	input := HistogramInput{Count: 100, Boundaries: []float64{50, 100}}

	result1, err1 := gen1.GenerateHistogram(input, func(rnd *rand.Rand, _ time.Time) float64 {
		return NormalRandom(rnd, 75, 10)
	})

	result2, err2 := gen2.GenerateHistogram(input, func(rnd *rand.Rand, _ time.Time) float64 {
		return NormalRandom(rnd, 75, 10)
	})

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected no errors, got %v, %v", err1, err2)
	}

	// With same seed, results should be identical
	if result1.Expected.Sum != result2.Expected.Sum {
		t.Errorf("Expected identical sums with same seed, got %.2f vs %.2f",
			result1.Expected.Sum, result2.Expected.Sum)
	}
}

// TestTimeBasedPatterns tests time-based value functions
func TestTimeBasedPatterns(t *testing.T) {
	gen := NewHistogramGenerator(GenerationOptions{Seed: 999})
	input := HistogramInput{Count: 100, Boundaries: []float64{25, 50, 75, 100}}

	patterns := []struct {
		name      string
		valueFunc func(*rand.Rand, time.Time) float64
	}{
		{
			name: "Sinusoidal",
			valueFunc: func(rnd *rand.Rand, t time.Time) float64 {
				return SinusoidalValue(rnd, t, 20, 3600, 0, 50)
			},
		},
		{
			name: "Spiky",
			valueFunc: func(rnd *rand.Rand, _ time.Time) float64 {
				return SpikyValue(rnd, 50, 100, 0.1)
			},
		},
		{
			name: "Trending",
			valueFunc: func(rnd *rand.Rand, t time.Time) float64 {
				return TrendingValue(rnd, t, 40, 0.001, 5)
			},
		},
	}

	for _, pattern := range patterns {
		t.Run(pattern.name, func(t *testing.T) {
			result, err := gen.GenerateHistogram(input, pattern.valueFunc)
			if err != nil {
				t.Fatalf("Pattern %s failed: %v", pattern.name, err)
			}

			if result.Expected.Count != 100 {
				t.Errorf("Pattern %s: expected 100 samples, got %d", pattern.name, result.Expected.Count)
			}

			// Test that we got reasonable values (not all zeros or infinities)
			if result.Expected.Sum <= 0 || result.Expected.Average <= 0 {
				t.Errorf("Pattern %s: got unreasonable values sum=%.2f, avg=%.2f",
					pattern.name, result.Expected.Sum, result.Expected.Average)
			}
		})
	}
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	gen := NewHistogramGenerator(GenerationOptions{Seed: 1})

	t.Run("ZeroCount", func(t *testing.T) {
		input := HistogramInput{Count: 0}
		result, err := gen.GenerateHistogram(input, nil)
		if err != nil {
			t.Fatalf("Expected no error with zero count, got %v", err)
		}

		// Should default to some reasonable count
		if result.Expected.Count == 0 {
			t.Error("Expected non-zero count when input count is 0")
		}
	})

	t.Run("NilValueFunc", func(t *testing.T) {
		input := HistogramInput{Count: 10}
		result, err := gen.GenerateHistogram(input, nil)
		if err != nil {
			t.Fatalf("Expected no error with nil value func, got %v", err)
		}

		if result.Expected.Count == 0 {
			t.Error("Expected samples even with nil value function")
		}
	})

	t.Run("EmptyBoundaries", func(t *testing.T) {
		input := HistogramInput{Count: 10, Boundaries: []float64{}}
		result, err := gen.GenerateHistogram(input, nil)
		if err != nil {
			t.Fatalf("Expected no error with empty boundaries, got %v", err)
		}

		// Should generate default boundaries
		if len(result.Input.Boundaries) == 0 {
			t.Error("Expected default boundaries when none provided")
		}
	})
}

func TestGenerateAccuracyDataset(_ *testing.T) {
	rng := rand.New(rand.NewPCG(0xFEEDBEEF, 0xFEEDBEEF))
	datapoints := make([]float64, 10000)
	for i := range 10000 {
		datapoints[i] = LogNormalRandom(rng, -4.894, 1.176)
	}
	slices.Sort(datapoints)
	for i, v := range datapoints {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%.3e", v)
	}
	fmt.Println()
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Helper function to create float64 pointers
func ptr(f float64) *float64 {
	return &f
}
