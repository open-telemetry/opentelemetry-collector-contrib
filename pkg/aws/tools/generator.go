// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/tools"

import (
	"math"
	"math/rand/v2"
	"slices"
	"sort"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"
)

type DistributionConfig struct {
	Name       string
	SampleSize int
	Params     map[string]float64
}

type GeneratedDataset struct {
	Name              string
	Data              []float64
	ActualPercentiles map[float64]float64
	Histograms        []histograms.HistogramTestCase
}

func GenerateTestData() []GeneratedDataset {
	return GenerateTestDataWithSeed(42)
}

func GenerateTestDataWithSeed(seed uint64) []GeneratedDataset {
	configs := []DistributionConfig{
		{Name: "LogNormal", SampleSize: 2000, Params: map[string]float64{"mu": 2.0, "sigma": 0.5}},
		{Name: "Weibull", SampleSize: 2000, Params: map[string]float64{"shape": 2.0, "scale": 100.0}},
		{Name: "Normal", SampleSize: 2000, Params: map[string]float64{"mean": 50.0, "stddev": 15.0}},
		{Name: "Gamma", SampleSize: 2000, Params: map[string]float64{"shape": 2.0, "rate": 0.1}},
	}

	var datasets []GeneratedDataset
	for _, config := range configs {
		// each dataset gets a copy of the same rand so that they don't interfere with each other
		rng := rand.New(rand.NewPCG(seed, seed))
		data := generateSamples(config, rng)
		percentiles := calculatePercentiles(data, []float64{0.01, 0.25, 0.5, 0.75, 0.99})
		histograms := createTestCases(config.Name, data, percentiles)

		datasets = append(datasets, GeneratedDataset{
			Name:              config.Name,
			Data:              data,
			ActualPercentiles: percentiles,
			Histograms:        histograms,
		})
	}

	return datasets
}

func generateSamples(config DistributionConfig, rng *rand.Rand) []float64 {
	data := make([]float64, config.SampleSize)

	switch config.Name {
	case "LogNormal":
		mu, sigma := config.Params["mu"], config.Params["sigma"]
		for i := 0; i < config.SampleSize; i++ {
			data[i] = math.Exp(rng.NormFloat64()*sigma + mu)
		}
	case "Weibull":
		shape, scale := config.Params["shape"], config.Params["scale"]
		for i := 0; i < config.SampleSize; i++ {
			u := rng.Float64()
			data[i] = scale * math.Pow(-math.Log(1-u), 1/shape)
		}
	case "Normal":
		mean, stddev := config.Params["mean"], config.Params["stddev"]
		for i := 0; i < config.SampleSize; i++ {
			data[i] = rng.NormFloat64()*stddev + mean
		}
	case "Gamma":
		shape, rate := config.Params["shape"], config.Params["rate"]
		for i := 0; i < config.SampleSize; i++ {
			data[i] = gammaRandom(shape, rng) / rate
		}
	}

	return data
}

// gammaRandom generates a random sample from a gamma distribution with the given shape parameter.
// It uses the Marsaglia and Tsang method (2000) for efficient gamma random number generation.
//
// For shape < 1, it uses the transformation property: if X ~ Gamma(shape+1, 1), then
// X * U^(1/shape) ~ Gamma(shape, 1) where U ~ Uniform(0,1).
//
// For shape >= 1, it uses the squeeze acceptance method which is highly efficient
// with an acceptance rate > 95% for most shape values.
//
// Parameters:
//   - shape: the shape parameter (α) of the gamma distribution, must be > 0
//   - rng: random number generator for sampling
//
// Returns: a random sample from Gamma(shape, 1) distribution
func gammaRandom(shape float64, rng *rand.Rand) float64 {
	if shape < 1 {
		return gammaRandom(shape+1, rng) * math.Pow(rng.Float64(), 1/shape)
	}

	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		x := rng.NormFloat64()
		v := 1.0 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

func calculatePercentiles(data []float64, percentiles []float64) map[float64]float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	result := make(map[float64]float64)
	for _, p := range percentiles {
		idx := int(p * float64(len(sorted)-1))
		result[p] = sorted[idx]
	}
	return result
}

func createTestCases(name string, data []float64, percentiles map[float64]float64) []histograms.HistogramTestCase {
	minimum := slices.Min(data)
	maximum := slices.Max(data)
	sum := sum(data)

	// Create different histogram configurations
	configs := []struct {
		suffix     string
		boundaries []float64
	}{
		{"Linear10", createLinearBoundaries(minimum, maximum, 10)},
		{"Linear20", createLinearBoundaries(minimum, maximum, 20)},
		{"Exponential", createExponentialBoundaries(minimum, maximum, 15)},
		{"Percentile", createPercentileBoundaries(data, []float64{0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99})},
	}

	var testCases []histograms.HistogramTestCase
	for _, config := range configs {
		counts := calculateCounts(data, config.boundaries)

		testCases = append(testCases, histograms.HistogramTestCase{
			Name: name + "_" + config.suffix,
			Input: histograms.HistogramInput{
				Count:      uint64(len(data)),
				Sum:        sum,
				Min:        &minimum,
				Max:        &maximum,
				Boundaries: config.boundaries,
				Counts:     counts,
				Attributes: map[string]string{"distribution": name, "config": config.suffix},
			},
			Expected: histograms.ExpectedMetrics{
				Count:            uint64(len(data)),
				Sum:              sum,
				Average:          sum / float64(len(data)),
				Min:              &minimum,
				Max:              &maximum,
				PercentileRanges: createPercentileRanges(percentiles, config.boundaries, minimum, maximum),
			},
		})
	}

	return testCases
}

func createLinearBoundaries(minimum, maximum float64, buckets int) []float64 {
	boundaries := make([]float64, buckets-1)
	step := (maximum - minimum) / float64(buckets)
	for i := 0; i < buckets-1; i++ {
		boundaries[i] = minimum + float64(i+1)*step
	}
	return boundaries
}

func createExponentialBoundaries(minimum, maximum float64, buckets int) []float64 {
	if minimum <= 0 {
		minimum = 0.1
	}
	boundaries := make([]float64, buckets-1)
	ratio := math.Pow(maximum/minimum, 1.0/float64(buckets))
	for i := 0; i < buckets-1; i++ {
		boundaries[i] = minimum * math.Pow(ratio, float64(i+1))
	}
	return boundaries
}

func createPercentileBoundaries(data []float64, percentiles []float64) []float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	boundaries := make([]float64, len(percentiles))
	for i, p := range percentiles {
		idx := int(p * float64(len(sorted)-1))
		boundaries[i] = sorted[idx]
	}
	return boundaries
}

func calculateCounts(data []float64, boundaries []float64) []uint64 {
	counts := make([]uint64, len(boundaries)+1)

	for _, value := range data {
		bucket := 0
		for i, boundary := range boundaries {
			if value <= boundary {
				bucket = i
				break
			}
			bucket = i + 1
		}
		counts[bucket]++
	}

	return counts
}

func createPercentileRanges(percentiles map[float64]float64, boundaries []float64, minimum, maximum float64) map[float64]histograms.PercentileRange {
	ranges := make(map[float64]histograms.PercentileRange)

	for p, value := range percentiles {
		low, high := minimum, maximum

		// Find bucket containing this percentile value
		for i, boundary := range boundaries {
			if value <= boundary {
				if i > 0 {
					low = boundaries[i-1]
				}
				high = boundary
				break
			}
			if i == len(boundaries)-1 {
				low = boundary
			}
		}

		ranges[p] = histograms.PercentileRange{Low: low, High: high}
	}

	return ranges
}

func sum(data []float64) float64 {
	total := 0.0
	for _, v := range data {
		total += v
	}
	return total
}
