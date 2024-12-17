// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package exphistogram contains utility functions for exponential histogram conversions.
package exphistogram // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/exphistogram"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// LowerBoundary calculates the lower boundary given index and scale.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func LowerBoundary(index, scale int) float64 {
	if scale <= 0 {
		return LowerBoundaryNegativeScale(index, scale)
	}
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return 2.0 * math.Exp(float64(index-(1<<scale))*inverseFactor)
}

// LowerBoundaryNegativeScale calculates the lower boundary for scale <= 0.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func LowerBoundaryNegativeScale(index, scale int) float64 {
	return math.Ldexp(1, index<<-scale)
}

// ToTDigest converts an OTLP exponential histogram data point to T-Digest counts and mean centroid values.
func ToTDigest(dp pmetric.ExponentialHistogramDataPoint) (counts []int64, values []float64) {
	scale := int(dp.Scale())

	offset := int(dp.Negative().Offset())
	bucketCounts := dp.Negative().BucketCounts()
	for i := bucketCounts.Len() - 1; i >= 0; i-- {
		count := bucketCounts.At(i)
		if count == 0 {
			continue
		}
		lb := -LowerBoundary(offset+i+1, scale)
		ub := -LowerBoundary(offset+i, scale)
		counts = append(counts, safeUint64ToInt64(count))
		values = append(values, lb+(ub-lb)/2)
	}

	if zeroCount := dp.ZeroCount(); zeroCount != 0 {
		counts = append(counts, safeUint64ToInt64(zeroCount))
		values = append(values, 0)
	}

	offset = int(dp.Positive().Offset())
	bucketCounts = dp.Positive().BucketCounts()
	for i := 0; i < bucketCounts.Len(); i++ {
		count := bucketCounts.At(i)
		if count == 0 {
			continue
		}
		lb := LowerBoundary(offset+i, scale)
		ub := LowerBoundary(offset+i+1, scale)
		counts = append(counts, safeUint64ToInt64(count))
		values = append(values, lb+(ub-lb)/2)
	}
	return
}

func safeUint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	} else {
		return int64(v) // nolint:goset // overflow checked
	}
}
