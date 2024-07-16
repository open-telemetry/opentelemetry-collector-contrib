// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expotest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

type Histogram struct {
	Ts pcommon.Timestamp

	Pos, Neg expo.Buckets
	PosNeg   expo.Buckets

	Scale int
	Count uint64
	Sum   *float64

	Min, Max *float64

	Zt float64
	Zc uint64
}

func (hist Histogram) Into() expo.DataPoint {
	dp := pmetric.NewExponentialHistogramDataPoint()
	dp.SetTimestamp(hist.Ts)

	if !zero(hist.PosNeg) {
		hist.PosNeg.CopyTo(dp.Positive())
		hist.PosNeg.CopyTo(dp.Negative())
	}

	if !zero(hist.Pos) {
		hist.Pos.MoveTo(dp.Positive())
	}
	if !zero(hist.Neg) {
		hist.Neg.MoveTo(dp.Negative())
	}

	dp.SetCount(hist.Count)
	if hist.Sum != nil {
		dp.SetSum(*hist.Sum)
	}

	if hist.Min != nil {
		dp.SetMin(*hist.Min)
	}
	if hist.Max != nil {
		dp.SetMax(*hist.Max)
	}

	dp.SetScale(int32(hist.Scale))
	dp.SetZeroThreshold(hist.Zt)
	dp.SetZeroCount(hist.Zc)
	return dp
}

func zero[T comparable](v T) bool {
	return v == *new(T)
}
