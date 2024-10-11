// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type builder struct {
	metric pmetric.Metric
	attrs  []string
}

// metricBuilder is used to build metrics for testing
func metricBuilder(metricType pmetric.MetricType, name string, attrs ...string) builder {
	m := pmetric.NewMetric()
	switch metricType {
	case pmetric.MetricTypeGauge:
		m.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		m.SetEmptySum()
	case pmetric.MetricTypeHistogram:
		m.SetEmptyHistogram()
	case pmetric.MetricTypeExponentialHistogram:
		m.SetEmptyExponentialHistogram()
	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
	}
	m.SetName(name)
	return builder{
		metric: m,
		attrs:  attrs,
	}
}

func (b builder) addDescription(description string) builder {
	b.metric.SetDescription(description)
	return b
}

func (b builder) addIntDatapoint(start, ts pcommon.Timestamp, val int64, attrValues ...string) builder {
	dp := b.addNumberDatapoint(start, ts, attrValues)
	dp.SetIntValue(val)
	return b
}

func (b builder) addDoubleDatapoint(start, ts pcommon.Timestamp, val float64, attrValues ...string) builder {
	dp := b.addNumberDatapoint(start, ts, attrValues)
	dp.SetDoubleValue(val)
	return b
}

func (b builder) setAttrs(attrs pcommon.Map, attrValues []string) {
	if len(attrValues) != len(b.attrs) {
		panic(
			fmt.Sprintf(
				"not enough attributes, expected %d attributes but got %s",
				len(b.attrs),
				attrValues),
		)
	}
	for i, a := range b.attrs {
		attrs.PutStr(a, attrValues[i])
	}
}

func (b builder) addNumberDatapoint(start, ts pcommon.Timestamp, attrValues []string) pmetric.NumberDataPoint {
	var dp pmetric.NumberDataPoint
	switch t := b.metric.Type(); t {
	case pmetric.MetricTypeGauge:
		dp = b.metric.Gauge().DataPoints().AppendEmpty()
	case pmetric.MetricTypeSum:
		dp = b.metric.Sum().DataPoints().AppendEmpty()
	default:
		panic(t.String())
	}
	b.setAttrs(dp.Attributes(), attrValues)
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	return dp
}

func (b builder) addHistogramDatapoint(start, ts pcommon.Timestamp, count uint64, sum float64, bounds []float64,
	buckets []uint64, attrValues ...string) builder {
	if b.metric.Type() != pmetric.MetricTypeHistogram {
		panic(b.metric.Type().String())
	}
	dp := b.metric.Histogram().DataPoints().AppendEmpty()
	b.setAttrs(dp.Attributes(), attrValues)
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.ExplicitBounds().FromRaw(bounds)
	dp.BucketCounts().FromRaw(buckets)
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	return b
}

func (b builder) addHistogramDatapointWithMinMaxAndExemplars(start, ts pcommon.Timestamp, count uint64, sum, min, max float64,
	bounds []float64, buckets []uint64, exemplarValues []float64, attrValues ...string) builder {
	if b.metric.Type() != pmetric.MetricTypeHistogram {
		panic(b.metric.Type().String())
	}
	dp := b.metric.Histogram().DataPoints().AppendEmpty()
	b.setAttrs(dp.Attributes(), attrValues)
	dp.SetCount(count)
	dp.SetSum(sum)
	dp.SetMin(min)
	dp.SetMax(max)
	dp.ExplicitBounds().FromRaw(bounds)
	dp.BucketCounts().FromRaw(buckets)
	for ei := 0; ei < len(exemplarValues); ei++ {
		exemplar := dp.Exemplars().AppendEmpty()
		exemplar.SetTimestamp(ts)
		exemplar.SetDoubleValue(exemplarValues[ei])
	}
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	return b
}

type expHistogramConfig struct {
	count          uint64
	sum            float64
	min            float64
	max            float64
	zeroThreshold  float64
	zeroCount      uint64
	scale          int32
	positiveOffset int32
	positiveCount  []uint64
	negativeOffset int32
	negativeCount  []uint64
	exemplarValues []float64
}

func (b builder) addExpHistogramDatapoint(config expHistogramConfig) builder {
	if b.metric.Type() != pmetric.MetricTypeExponentialHistogram {
		panic(b.metric.Type().String())
	}
	dp := b.metric.ExponentialHistogram().DataPoints().AppendEmpty()
	dp.SetCount(config.count)
	dp.SetSum(config.sum)
	dp.SetMin(config.min)
	dp.SetMax(config.max)
	dp.SetZeroThreshold(config.zeroThreshold)
	dp.SetZeroCount(config.zeroCount)
	dp.SetScale(config.scale)
	dp.Positive().SetOffset(config.positiveOffset)
	dp.Positive().BucketCounts().FromRaw(config.positiveCount)
	dp.Negative().SetOffset(config.negativeOffset)
	dp.Negative().BucketCounts().FromRaw(config.negativeCount)
	for ei := 0; ei < len(config.exemplarValues); ei++ {
		exemplar := dp.Exemplars().AppendEmpty()
		exemplar.SetTimestamp(1)
		exemplar.SetDoubleValue(config.exemplarValues[ei])
	}
	dp.SetStartTimestamp(1)
	dp.SetTimestamp(1)
	return b
}

func buildExpHistogramBucket(m map[int]uint64) []uint64 {
	if len(m) == 0 {
		return []uint64{}
	}
	maxIndex := 0
	for index := range m {
		if index > maxIndex {
			maxIndex = index
		}
	}

	result := make([]uint64, maxIndex+1)
	for index, count := range m {
		result[index] = count
	}

	return result
}

// setUnit sets the unit of this metric
func (b builder) setUnit(unit string) builder {
	b.metric.SetUnit(unit)
	return b
}

// Build builds from the builder to the final metric
func (b builder) build() pmetric.Metric {
	return b.metric
}
