// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Point[Self any] interface {
	StartTimestamp() pcommon.Timestamp
	Timestamp() pcommon.Timestamp
	Attributes() pcommon.Map

	Clone() Self
	CopyTo(Self)

	Add(Self) Self
}

type Number struct {
	pmetric.NumberDataPoint
}

func (dp Number) Clone() Number {
	clone := Number{NumberDataPoint: pmetric.NewNumberDataPoint()}
	if dp.NumberDataPoint != (pmetric.NumberDataPoint{}) {
		dp.CopyTo(clone)
	}
	return clone
}

func (dp Number) CopyTo(dst Number) {
	dp.NumberDataPoint.CopyTo(dst.NumberDataPoint)
}

type Histogram struct {
	pmetric.HistogramDataPoint
}

func (dp Histogram) Clone() Histogram {
	clone := Histogram{HistogramDataPoint: pmetric.NewHistogramDataPoint()}
	if dp.HistogramDataPoint != (pmetric.HistogramDataPoint{}) {
		dp.CopyTo(clone)
	}
	return clone
}

func (dp Histogram) CopyTo(dst Histogram) {
	dp.HistogramDataPoint.CopyTo(dst.HistogramDataPoint)
}

type ExpHistogram struct {
	pmetric.ExponentialHistogramDataPoint
}

func (dp ExpHistogram) Clone() ExpHistogram {
	clone := ExpHistogram{ExponentialHistogramDataPoint: pmetric.NewExponentialHistogramDataPoint()}
	if dp.ExponentialHistogramDataPoint != (pmetric.ExponentialHistogramDataPoint{}) {
		dp.CopyTo(clone)
	}
	return clone
}

func (dp ExpHistogram) CopyTo(dst ExpHistogram) {
	dp.ExponentialHistogramDataPoint.CopyTo(dst.ExponentialHistogramDataPoint)
}

type mustPoint[D Point[D]] struct{ _ D }

var (
	_ = mustPoint[Number]{}
	_ = mustPoint[Histogram]{}
	_ = mustPoint[ExpHistogram]{}
)
