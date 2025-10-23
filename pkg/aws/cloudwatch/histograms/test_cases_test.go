// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package histograms // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHistogramFeasibility(t *testing.T) {
	testCases := TestCases()
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc.Input)
			assert.True(t, feasible, reason)

			// check that the test case percentile ranges are valid
			for percentile, expectedRange := range tc.Expected.PercentileRanges {
				calculatedLow, calculatedHigh := calculatePercentileRange(tc.Input, percentile)
				assert.Equal(t, expectedRange.Low, calculatedLow, "calculated low does not match expected low for percentile %v", percentile)
				assert.Equal(t, expectedRange.High, calculatedHigh, "calculated high does not match expected high for percentile %v", percentile)
			}

			assertOptionalFloat(t, "min", tc.Expected.Min, tc.Input.Min)
			assertOptionalFloat(t, "max", tc.Expected.Max, tc.Input.Max)
			assert.InDelta(t, tc.Expected.Average, tc.Input.Sum/float64(tc.Input.Count), 0.01)
		})
	}
}

func TestInvalidHistogramFeasibility(t *testing.T) {
	invalidTestCases := InvalidTestCases()

	for _, tc := range invalidTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			feasible, reason := checkFeasibility(tc.Input)
			assert.False(t, feasible, reason)
		})
	}
}

func TestVisualizeHistograms(t *testing.T) {
	// uncomment the next line to visualize the input histograms
	t.Skip("Skip visualization test")
	testCases := TestCases()
	for _, tc := range testCases {
		t.Run(tc.Name, func(_ *testing.T) {
			// The large bucket tests are just too big to output
			if matched, _ := regexp.MatchString("\\d\\d\\d Buckets", tc.Name); matched {
				return
			}
			visualizeHistogramWithPercentiles(tc.Input)
		})
	}
}

func checkFeasibility(histogramInput HistogramInput) (bool, string) {
	lenBoundaries := len(histogramInput.Boundaries)
	lenCounts := len(histogramInput.Counts)

	// Check counts length matches boundaries + 1
	// Special case: empty histogram is valid
	if lenBoundaries != 0 && lenCounts != 0 && lenCounts != lenBoundaries+1 {
		return false, "Can't have counts without boundaries"
	}

	if histogramInput.Max != nil && histogramInput.Min != nil && *histogramInput.Min > *histogramInput.Max {
		return false, fmt.Sprintf("min %f is greater than max %f", *histogramInput.Min, *histogramInput.Max)
	}

	if histogramInput.Max != nil {
		if err := checkNanInf(*histogramInput.Max, "max"); err != nil {
			return false, err.Error()
		}
	}

	if histogramInput.Min != nil {
		if err := checkNanInf(*histogramInput.Min, "min"); err != nil {
			return false, err.Error()
		}
	}

	if err := checkNanInf(histogramInput.Sum, "sum"); err != nil {
		return false, err.Error()
	}

	for _, bound := range histogramInput.Boundaries {
		if err := checkNanInf(bound, "boundary"); err != nil {
			return false, err.Error()
		}
	}

	// Rest of checks only apply if we have boundaries/counts
	if lenBoundaries > 0 || lenCounts > 0 {
		// Check boundaries are in ascending order
		for i := 1; i < lenBoundaries; i++ {
			if histogramInput.Boundaries[i] <= histogramInput.Boundaries[i-1] {
				return false, fmt.Sprintf("boundaries not in ascending order: %v <= %v",
					histogramInput.Boundaries[i], histogramInput.Boundaries[i-1])
			}
		}

		// Check counts array length
		if lenCounts != lenBoundaries+1 {
			return false, fmt.Sprintf("counts length (%d) should be boundaries length (%d) + 1",
				lenCounts, lenBoundaries)
		}

		// Verify total count matches sum of bucket counts
		var totalCount uint64
		for _, count := range histogramInput.Counts {
			totalCount += count
		}
		if totalCount != histogramInput.Count {
			return false, fmt.Sprintf("sum of counts (%d) doesn't match total count (%d)",
				totalCount, histogramInput.Count)
		}

		// Check min/max feasibility if defined
		if histogramInput.Min != nil {
			// If there are boundaries, first bucket must have counts > 0 only if min <= first boundary
			if lenBoundaries > 0 && histogramInput.Counts[0] > 0 && *histogramInput.Min > histogramInput.Boundaries[0] {
				return false, fmt.Sprintf("min (%v) > first boundary (%v) but first bucket has counts",
					*histogramInput.Min, histogramInput.Boundaries[0])
			}
		}

		if histogramInput.Max != nil {
			// If there are boundaries, last bucket must have counts > 0 only if max > last boundary
			if lenBoundaries > 0 && histogramInput.Counts[lenCounts-1] > 0 &&
				*histogramInput.Max <= histogramInput.Boundaries[lenBoundaries-1] {
				return false, fmt.Sprintf("max (%v) <= last boundary (%v) but overflow bucket has counts",
					*histogramInput.Max, histogramInput.Boundaries[lenBoundaries-1])
			}
		}

		// Check sum feasibility
		if lenBoundaries > 0 {
			// Calculate minimum possible sum
			minSum := float64(0)
			if histogramInput.Min != nil {
				// Find which bucket the minimum value belongs to
				minBucket := 0
				for i, bound := range histogramInput.Boundaries {
					if *histogramInput.Min > bound {
						minBucket = i + 1
					}
				}
				// Apply min value only from its containing bucket
				for i := minBucket; i < lenCounts; i++ {
					if i == minBucket {
						minSum += float64(histogramInput.Counts[i]) * *histogramInput.Min
					} else {
						minSum += float64(histogramInput.Counts[i]) * histogramInput.Boundaries[i-1]
					}
				}
			} else {
				// Without min, use lower bounds
				for i := 1; i < lenCounts; i++ {
					minSum += float64(histogramInput.Counts[i]) * histogramInput.Boundaries[i-1]
				}
			}

			// Calculate maximum possible sum
			maxSum := float64(0)
			if histogramInput.Max != nil {
				// Find which bucket the maximum value belongs to
				maxBucket := lenBoundaries // Default to overflow bucket
				for i, bound := range histogramInput.Boundaries {
					if *histogramInput.Max <= bound {
						maxBucket = i
						break
					}
				}
				// Apply max value only up to its containing bucket
				for i := 0; i < lenCounts; i++ {
					switch {
					case i > maxBucket:
						maxSum += float64(histogramInput.Counts[i]) * *histogramInput.Max
					case i == lenBoundaries:
						maxSum += float64(histogramInput.Counts[i]) * *histogramInput.Max
					default:
						maxSum += float64(histogramInput.Counts[i]) * histogramInput.Boundaries[i]
					}
				}
			} else {
				// If no max defined, we can't verify upper bound
				maxSum = math.Inf(1)
			}

			if histogramInput.Sum < minSum {
				return false, fmt.Sprintf("sum (%v) is less than minimum possible sum (%v)",
					histogramInput.Sum, minSum)
			}
			if maxSum != math.Inf(1) && histogramInput.Sum > maxSum {
				return false, fmt.Sprintf("sum (%v) is greater than maximum possible sum (%v)",
					histogramInput.Sum, maxSum)
			}
		}
	}

	return true, ""
}

func calculatePercentileRange(hi HistogramInput, percentile float64) (float64, float64) {
	if len(hi.Boundaries) == 0 {
		// No buckets - use min/max if available
		if hi.Min != nil && hi.Max != nil {
			return *hi.Min, *hi.Max
		}
		return math.Inf(-1), math.Inf(1)
	}

	percentilePosition := max(1, uint64(math.Round(float64(hi.Count)*percentile)))
	var cumulativeCount uint64

	// Find which bucket contains the percentile
	for i, count := range hi.Counts {
		cumulativeCount += count
		if cumulativeCount >= percentilePosition {
			switch {
			// Found the bucket containing the percentile
			case i == 0:
				// First bucket: (-inf, bounds[0]]
				if hi.Min != nil {
					return *hi.Min, hi.Boundaries[0]
				}
				return math.Inf(-1), hi.Boundaries[0]
			case i == len(hi.Boundaries):
				// Last bucket: (bounds[last], +inf)
				if hi.Max != nil {
					return hi.Boundaries[i-1], *hi.Max
				}
				return hi.Boundaries[i-1], math.Inf(1)
			default:
				// Middle bucket: (bounds[i-1], bounds[i]]
				return hi.Boundaries[i-1], hi.Boundaries[i]
			}
		}
	}
	return 0, 0 // Should never reach here for valid histograms
}

func assertOptionalFloat(t *testing.T, name string, expected, actual *float64) {
	if expected != nil {
		assert.NotNil(t, actual, "Expected %s defined but not defined on input", name)
		if actual != nil {
			assert.InDelta(t, *expected, *actual, 0.0001)
		}
	} else {
		assert.Nil(t, actual, "Input %s defined but no %s is expected", name, name)
	}
}

func visualizeHistogramWithPercentiles(hi HistogramInput) {
	fmt.Printf("\nHistogram Visualization with Percentiles\n")
	fmt.Printf("Count: %d, Sum: %.2f\n", hi.Count, hi.Sum)
	if hi.Min != nil {
		fmt.Printf("Min: %.2f ", *hi.Min)
	}
	if hi.Max != nil {
		fmt.Printf("Max: %.2f", *hi.Max)
	}
	fmt.Println()

	if len(hi.Boundaries) == 0 {
		fmt.Println("No buckets defined")
		return
	}

	// Calculate cumulative counts for CDF
	cumulativeCounts := make([]uint64, len(hi.Counts))
	var total uint64
	for i, count := range hi.Counts {
		total += count
		cumulativeCounts[i] = total
	}

	// Find percentile positions
	percentiles := []float64{0.01, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99}
	percentilePositions := make(map[float64]int)
	for _, p := range percentiles {
		pos := max(1, uint64(math.Round(float64(hi.Count)*p)))
		for i, cumCount := range cumulativeCounts {
			if cumCount >= pos {
				percentilePositions[p] = i
				break
			}
		}
	}

	maxCount := uint64(0)
	for _, count := range hi.Counts {
		if count > maxCount {
			maxCount = count
		}
	}

	fmt.Println("\nHistogram:")
	for i, count := range hi.Counts {
		var bucketLabel string
		switch {
		case i == 0:
			if hi.Min != nil {
				bucketLabel = fmt.Sprintf("(%.2f, %.1f]", *hi.Min, hi.Boundaries[0])
			} else {
				bucketLabel = fmt.Sprintf("(-∞, %.1f]", hi.Boundaries[0])
			}
		case i == len(hi.Boundaries):
			if hi.Max != nil {
				bucketLabel = fmt.Sprintf("(%.1f, %.2f]", hi.Boundaries[i-1], *hi.Max)
			} else {
				bucketLabel = fmt.Sprintf("(%.1f, +∞)", hi.Boundaries[i-1])
			}
		default:
			bucketLabel = fmt.Sprintf("(%.1f, %.1f]", hi.Boundaries[i-1], hi.Boundaries[i])
		}

		barLength := int(float64(count) / float64(maxCount) * 40)
		bar := strings.Repeat("█", barLength)

		// Mark percentile buckets
		percentileMarkers := ""
		for _, p := range percentiles {
			if percentilePositions[p] == i {
				percentileMarkers += fmt.Sprintf(" P%.0f", p*100)
			}
		}

		fmt.Printf("%-30s %4d |%s%s\n", bucketLabel, count, bar, percentileMarkers)
	}

	fmt.Println("\nCumulative Distribution (CDF):")
	for i, cumCount := range cumulativeCounts {
		var bucketLabel string
		switch {
		case i == 0:
			bucketLabel = fmt.Sprintf("≤ %.1f", hi.Boundaries[0])
		case i == len(hi.Boundaries):
			bucketLabel = "≤ +∞"
		default:
			bucketLabel = fmt.Sprintf("≤ %.1f", hi.Boundaries[i])
		}

		cdfPercent := float64(cumCount) / float64(hi.Count) * 100
		cdfBarLength := int(cdfPercent / 100 * 40)
		cdfBar := strings.Repeat("▓", cdfBarLength)

		// Add percentile lines
		percentileLines := ""
		for _, p := range percentiles {
			if percentilePositions[p] == i {
				percentileLines += fmt.Sprintf(" ──P%.0f", p*100)
			}
		}

		fmt.Printf("%-15s %6.1f%% |%s%s\n", bucketLabel, cdfPercent, cdfBar, percentileLines)
	}

	// Show percentile ranges
	fmt.Println("\nPercentile Ranges:")
	for _, p := range percentiles {
		low, high := calculatePercentileRange(hi, p)
		fmt.Printf("P%.0f: [%.2f, %.2f]\n", p*100, low, high)
	}
}
