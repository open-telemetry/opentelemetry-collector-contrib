// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package histograms // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"

import (
	"errors"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func CheckValidity(dp pmetric.HistogramDataPoint) error {
	errs := []error{}

	bounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()

	// Check counts length matches boundaries + 1
	// special case: no bucketCounts and no boundaries is still valid
	if bucketCounts.Len() != bounds.Len()+1 && bucketCounts.Len() != 0 && bounds.Len() != 0 {
		errs = append(errs, fmt.Errorf("bucket counts length (%d) doesn't match boundaries length (%d) + 1",
			bucketCounts.Len(), bounds.Len()))
	}

	if dp.HasMax() && dp.HasMin() && dp.Min() > dp.Max() {
		errs = append(errs, fmt.Errorf("min %f is greater than max %f", dp.Min(), dp.Max()))
	}

	if dp.HasMax() {
		errs = append(errs, checkNanInf(dp.Max(), "max"))
	}
	if dp.HasMin() {
		errs = append(errs, checkNanInf(dp.Min(), "min"))
	}
	if dp.HasSum() {
		errs = append(errs, checkNanInf(dp.Sum(), "sum"))
	}

	if bounds.Len() > 0 {
		// Check boundaries are in ascending order
		for i := 1; i < bounds.Len(); i++ {
			if bounds.At(i) <= bounds.At(i-1) {
				errs = append(errs, fmt.Errorf("boundaries not in ascending order: bucket index %d (%v) <= bucket index %d %v",
					i, bounds.At(i), i-1, bounds.At(i-1)))
			}
			errs = append(errs, checkNanInf(bounds.At(i), fmt.Sprintf("boundary %d", i)))
		}
	}

	return errors.Join(errs...)
}

func checkNanInf(value float64, name string) error {
	errs := []error{}
	if math.IsNaN(value) {
		errs = append(errs, errors.New(name+" is NaN"))
	}
	if math.IsInf(value, 0) {
		errs = append(errs, errors.New(name+" is +/-inf"))
	}
	return errors.Join(errs...)
}
