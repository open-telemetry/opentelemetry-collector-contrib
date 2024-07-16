// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type DataPointSlice[DP DataPoint[DP]] interface {
	Len() int
	At(i int) DP
	AppendEmpty() DP
}

type DataPoint[Self any] interface {
	pmetric.NumberDataPoint | pmetric.HistogramDataPoint | pmetric.ExponentialHistogramDataPoint

	Timestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	CopyTo(dest Self)
}
