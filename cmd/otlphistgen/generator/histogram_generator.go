// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math"
	rand "math/rand/v2"
	"sort"
	"time"
)

// HistogramGenerator generates histogram test cases using statistical distributions
type HistogramGenerator struct {
	rand     *rand.Rand
	endpoint string
}

// NewHistogramGenerator creates a new histogram generator with deterministic seed
func NewHistogramGenerator(opt ...GenerationOptions) *HistogramGenerator {
	seed := time.Now().UnixNano()
	var endpoint string

	if len(opt) > 0 {
		if opt[0].Seed != 0 {
			seed = opt[0].Seed
		}
		endpoint = opt[0].Endpoint
	}

	return &HistogramGenerator{
		rand:     rand.New(rand.NewPCG(uint64(seed), uint64(seed))),
		endpoint: endpoint,
	}
}

// GenerateHistogram generates histogram data from individual values using a value function
func (g *HistogramGenerator) GenerateHistogram(input HistogramInput, valueFunc func(*rand.Rand, time.Time) float64) (HistogramResult, error) {
	timestamp := time.Now()
	sampleCount := int(input.Count)

	if sampleCount <= 0 {
		sampleCount = 1000 // default sample count
	}

	// Generate individual values using the value function
	values := make([]float64, sampleCount)
	for i := 0; i < sampleCount; i++ {
		if valueFunc != nil {
			values[i] = valueFunc(g.rand, timestamp)
		} else {
			values[i] = g.rand.Float64() * 100 // default random value
		}
	}

	// Sort values to find min/max
	sort.Float64s(values)

	// Calculate basic stats
	var sum float64
	for _, v := range values {
		sum += v
	}

	generatedMin := values[0]
	generatedMax := values[len(values)-1]
	average := sum / float64(len(values))

	// Determine final min/max values
	var finalMin, finalMax float64
	if input.Min != nil {
		finalMin = *input.Min
	} else {
		finalMin = generatedMin
	}
	if input.Max != nil {
		finalMax = *input.Max
	} else {
		finalMax = generatedMax
	}

	// Use provided boundaries or generate them based on min/max
	boundaries := input.Boundaries
	if len(boundaries) == 0 {
		boundaries = generateBoundariesBetween(finalMin, finalMax, 10)
	}

	counts := make([]uint64, len(boundaries)+1)
	for _, value := range values {
		bucketIndex := len(boundaries) // default to overflow bucket
		for i, boundary := range boundaries {
			if value <= boundary {
				bucketIndex = i
				break
			}
		}
		counts[bucketIndex]++
	}

	// Calculate percentile ranges
	percentileRanges := g.calculatePercentileRangesFromValues(values, boundaries)

	// Use input min/max if provided, otherwise use generated values
	var resultMin, resultMax *float64
	if input.Min != nil {
		resultMin = input.Min
	} else {
		resultMin = &generatedMin
	}
	if input.Max != nil {
		resultMax = input.Max
	} else {
		resultMax = &generatedMax
	}

	generatedInput := HistogramInput{
		Count:      uint64(len(values)),
		Sum:        sum,
		Min:        resultMin,
		Max:        resultMax,
		Boundaries: boundaries,
		Counts:     counts,
		Attributes: input.Attributes,
	}

	expected := ExpectedMetrics{
		Count:            uint64(len(values)),
		Sum:              sum,
		Average:          average,
		Min:              resultMin,
		Max:              resultMax,
		PercentileRanges: percentileRanges,
	}

	return HistogramResult{
		Input:    generatedInput,
		Expected: expected,
	}, nil
}

// GenerateAndPublishHistograms generates and optionally publishes histogram data
func (g *HistogramGenerator) GenerateAndPublishHistograms(input HistogramInput, valueFunc func(*rand.Rand, time.Time) float64) (HistogramResult, error) {
	res, err := g.GenerateHistogram(input, valueFunc)
	if err != nil {
		return HistogramResult{}, err
	}

	if g.endpoint == "" {
		return res, nil
	}

	publisher := NewOTLPPublisher(g.endpoint)
	err = publisher.SendHistogramMetric("TelemetryGen", res)
	if err != nil {
		return HistogramResult{}, err
	}

	return res, nil
}

// calculatePercentileRangesFromValues calculates percentile ranges for sorted values
func (g *HistogramGenerator) calculatePercentileRangesFromValues(sortedValues []float64, boundaries []float64) map[float64]PercentileRange {
	percentiles := []float64{0.01, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99}
	ranges := make(map[float64]PercentileRange)

	for _, p := range percentiles {
		index := int(p * float64(len(sortedValues)))
		if index >= len(sortedValues) {
			index = len(sortedValues) - 1
		}

		value := sortedValues[index]

		// Find which bucket this percentile value falls into
		var low, high float64

		// Check if value falls in any boundary bucket
		bucketFound := false
		for i, boundary := range boundaries {
			if value <= boundary {
				if i > 0 {
					low = boundaries[i-1]
				} else {
					low = math.Inf(-1)
				}
				high = boundary
				bucketFound = true
				break
			}
		}

		// If not found in any boundary bucket, it's in the overflow bucket
		if !bucketFound {
			if len(boundaries) > 0 {
				low = boundaries[len(boundaries)-1]
			} else {
				low = math.Inf(-1)
			}
			high = math.Inf(1)
		}

		ranges[p] = PercentileRange{Low: low, High: high}
	}

	return ranges
}

// generateBoundariesBetween creates evenly spaced boundaries between min and max
func generateBoundariesBetween(minimum, maximum float64, numBuckets int) []float64 {
	if numBuckets <= 0 {
		numBuckets = 10
	}

	boundaries := make([]float64, numBuckets-1)
	step := (maximum - minimum) / float64(numBuckets)

	for i := 0; i < numBuckets-1; i++ {
		boundaries[i] = minimum + float64(i+1)*step
	}

	return boundaries
}
