// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSumDP_IntOnly(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key", "val")
	dp := newSumDP(attrs, false)

	dp.AggregateInt(10)
	dp.AggregateInt(20)
	dp.AggregateInt(-5)

	dest := pmetric.NewNumberDataPoint()
	now := time.Now()
	dp.Copy(now, dest)

	assert.Equal(t, pmetric.NumberDataPointValueTypeInt, dest.ValueType())
	assert.Equal(t, int64(25), dest.IntValue())
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dest.Timestamp())
	v, ok := dest.Attributes().Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", v.Str())
}

func TestSumDP_DoubleOnly(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key", "val")
	dp := newSumDP(attrs, true)

	dp.AggregateDouble(10.5)
	dp.AggregateDouble(20.25)
	dp.AggregateDouble(-5.5)

	dest := pmetric.NewNumberDataPoint()
	now := time.Now()
	dp.Copy(now, dest)

	assert.Equal(t, pmetric.NumberDataPointValueTypeDouble, dest.ValueType())
	assert.Equal(t, 25.25, dest.DoubleValue())
	assert.Equal(t, pcommon.NewTimestampFromTime(now), dest.Timestamp())
	v, ok := dest.Attributes().Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", v.Str())
}

func TestSumDP_IntThenDouble(t *testing.T) {
	attrs := pcommon.NewMap()
	dp := newSumDP(attrs, false)

	dp.AggregateInt(10)
	assert.NotPanics(t, func() {
		dp.AggregateDouble(2.5)
	})

	dest := pmetric.NewNumberDataPoint()
	dp.Copy(time.Now(), dest)

	assert.Equal(t, pmetric.NumberDataPointValueTypeDouble, dest.ValueType())
	assert.Equal(t, 12.5, dest.DoubleValue())
}

func TestSumDP_DoubleThenInt(t *testing.T) {
	attrs := pcommon.NewMap()
	dp := newSumDP(attrs, true)

	dp.AggregateDouble(2.5)
	assert.NotPanics(t, func() {
		dp.AggregateInt(10)
	})

	dest := pmetric.NewNumberDataPoint()
	dp.Copy(time.Now(), dest)

	assert.Equal(t, pmetric.NumberDataPointValueTypeDouble, dest.ValueType())
	assert.Equal(t, 12.5, dest.DoubleValue())
}

func TestSumDP_Interleaved(t *testing.T) {
	attrs := pcommon.NewMap()
	dp := newSumDP(attrs, false)

	dp.AggregateInt(5)
	dp.AggregateDouble(1.5)
	dp.AggregateInt(10)
	dp.AggregateDouble(3.5)

	dest := pmetric.NewNumberDataPoint()
	dp.Copy(time.Now(), dest)

	assert.Equal(t, pmetric.NumberDataPointValueTypeDouble, dest.ValueType())
	assert.Equal(t, 20.0, dest.DoubleValue())
}
