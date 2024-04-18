package expotest

import (
	"testing"

	"github.com/matryer/is"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
		is.I.Equal(want.ZeroCount(), got.ZeroCount())         // zero-count
		is.I.Equal(want.ZeroThreshold(), got.ZeroThreshold()) // zero-threshold
		is.Equal(want.Positive(), got.Positive())
		is.Equal(want.Negative(), got.Negative())
	case pmetric.ExponentialHistogramDataPointBuckets:
		want := want.(pmetric.ExponentialHistogramDataPointBuckets)
		is.I.Equal(want.Offset(), got.Offset())                             // offset
		is.I.Equal(want.BucketCounts().AsRaw(), got.BucketCounts().AsRaw()) // counts
	default:
		is.I.Equal(got, want)
	}
}
