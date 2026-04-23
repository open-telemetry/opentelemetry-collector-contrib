// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// sumDP counts the number of events (supports all event types)
type sumDP struct {
	attrs pcommon.Map

	isDbl  bool
	intVal int64
	dblVal float64
}

func newSumDP(attrs pcommon.Map, isDbl bool) *sumDP {
	return &sumDP{
		isDbl: isDbl,
		attrs: attrs,
	}
}

func (dp *sumDP) AggregateInt(v int64) {
	if dp.isDbl {
		panic("unexpected usage of sum datapoint, only integer value expected")
	}
	dp.intVal += v
}

func (dp *sumDP) AggregateDouble(v float64) {
	if !dp.isDbl {
		panic("unexpected usage of sum datapoint, only double value expected")
	}
	dp.dblVal += v
}

func (dp *sumDP) Copy(
	timestamp time.Time,
	dest pmetric.NumberDataPoint,
) {
	dp.attrs.CopyTo(dest.Attributes())
	if dp.isDbl {
		dest.SetDoubleValue(dp.dblVal)
	} else {
		dest.SetIntValue(dp.intVal)
	}
	// TODO determine appropriate start time
	dest.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
}
