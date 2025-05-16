// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// gaugeDP is a data point for gauge metrics.
type gaugeDP struct {
	attrs pcommon.Map
	val   pcommon.Value
}

func newGaugeDP(attrs pcommon.Map) *gaugeDP {
	return &gaugeDP{
		attrs: attrs,
	}
}

func (dp *gaugeDP) Aggregate(v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeDouble, pcommon.ValueTypeInt:
		dp.val = v
	default:
		panic("unexpected usage of gauge datapoint, only double or int value expected")
	}
}

// Copy copies the gauge data point to the destination number data point
func (dp *gaugeDP) Copy(
	timestamp time.Time,
	dest pmetric.NumberDataPoint,
) {
	dp.attrs.CopyTo(dest.Attributes())
	switch dp.val.Type() {
	case pcommon.ValueTypeDouble:
		dest.SetDoubleValue(dp.val.Double())
	case pcommon.ValueTypeInt:
		dest.SetIntValue(dp.val.Int())
	}
	// TODO determine appropriate start time
	dest.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
}
