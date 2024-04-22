// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expotest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"

import (
	"testing"

	"github.com/matryer/is"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

type I struct {
	*is.I
}

func Is(t *testing.T) *I {
	return &I{I: is.NewRelaxed(t)}
}

func (is *I) Equal(want, got any) {
	switch got := got.(type) {
	case expo.DataPoint:
		want := want.(expo.DataPoint)
		is.I.Equal(want.Timestamp(), got.Timestamp())           // time
		is.I.Equal(want.StartTimestamp(), got.StartTimestamp()) // start-time
		is.I.Equal(want.Scale(), got.Scale())                   // scale
		is.I.Equal(want.Count(), got.Count())                   // count
		is.I.Equal(want.Sum(), got.Sum())                       // sum
		is.I.Equal(want.ZeroCount(), got.ZeroCount())           // zero-count
		is.I.Equal(want.ZeroThreshold(), got.ZeroThreshold())   // zero-threshold
		is.Equal(want.Positive(), got.Positive())
		is.Equal(want.Negative(), got.Negative())
		is.NoErr(pmetrictest.CompareExponentialHistogramDataPoint(want, got)) // pmetrictest
	case pmetric.ExponentialHistogramDataPointBuckets:
		want := want.(pmetric.ExponentialHistogramDataPointBuckets)
		is.I.Equal(want.Offset(), got.Offset())                             // offset
		is.I.Equal(want.BucketCounts().AsRaw(), got.BucketCounts().AsRaw()) // counts
	default:
		is.I.Equal(got, want)
	}
}
