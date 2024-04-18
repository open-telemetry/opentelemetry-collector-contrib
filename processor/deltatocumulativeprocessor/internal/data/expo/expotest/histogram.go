package expotest

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Histogram struct {
	Pos, Neg Bins
	PosNeg   Bins

	Scale int

	Zt float64
	Zc uint64
}

func DataPoint(hist Histogram) expo.DataPoint {
	dp := pmetric.NewExponentialHistogramDataPoint()

	posneg := Buckets(hist.PosNeg)
	posneg.CopyTo(dp.Positive())
	posneg.CopyTo(dp.Negative())

	if (hist.Pos != Bins{}) {
		Buckets(hist.Pos).MoveTo(dp.Positive())
	}
	if (hist.Neg != Bins{}) {
		Buckets(hist.Neg).MoveTo(dp.Negative())
	}

	dp.SetScale(int32(hist.Scale))
	dp.SetZeroThreshold(hist.Zt)
	dp.SetZeroCount(hist.Zc)
	return dp
}
