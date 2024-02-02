// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"hash"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Stream struct {
	metric Metric
	attrs  [16]byte
}

func (i Stream) Hash() hash.Hash64 {
	sum := i.metric.Hash()
	sum.Write(i.attrs[:])
	return sum
}

func OfStream[DataPoint attrPoint](m Metric, dp DataPoint) Stream {
	return Stream{metric: m, attrs: pdatautil.MapHash(dp.Attributes())}
}

type attrPoint interface {
	pmetric.NumberDataPoint | pmetric.HistogramDataPoint | pmetric.ExponentialHistogramDataPoint | pmetric.SummaryDataPoint
	Attributes() pcommon.Map
}
