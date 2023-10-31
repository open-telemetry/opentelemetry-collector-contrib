// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"fmt"
	"sort"
	"strings"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func serializeSumPoint(name, prefix string, dims dimensions.NormalizedDimensionList, t pmetric.AggregationTemporality, dp pmetric.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	switch t {
	case pmetric.AggregationTemporalityCumulative:
		return serializeCumulativeCounter(name, prefix, dims, dp, prev)
	// for now unspecified is treated as delta
	case pmetric.AggregationTemporalityUnspecified:
		fallthrough
	case pmetric.AggregationTemporalityDelta:
		return serializeDeltaCounter(name, prefix, dims, dp)
	}

	return "", nil
}

func serializeSum(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions dimensions.NormalizedDimensionList, staticDimensions dimensions.NormalizedDimensionList, prev *ttlmap.TTLMap, metricLines []string) []string {
	sum := metric.Sum()

	if !sum.IsMonotonic() && sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
		logger.Warn(
			"dropping delta non-monotonic sum",
			zap.String("name", metric.Name()),
		)
		return metricLines
	}

	points := metric.Sum().DataPoints()

	for i := 0; i < points.Len(); i++ {
		dp := points.At(i)
		if sum.IsMonotonic() {
			// serialize monotonic sum points as count (cumulatives are converted to delta in serializeSumPoint)
			line, err := serializeSumPoint(
				metric.Name(),
				prefix,
				makeCombinedDimensions(defaultDimensions, dp.Attributes(), staticDimensions),
				metric.Sum().AggregationTemporality(),
				dp,
				prev,
			)

			if err != nil {
				logger.Warn(
					"Error serializing sum data point",
					zap.String("name", metric.Name()),
					zap.String("value-type", dp.ValueType().String()),
					zap.Error(err),
				)
			}

			if line != "" {
				metricLines = append(metricLines, line)
			}
		} else {
			// Cumulative non-monotonic sum points are serialized as gauges. Delta non-monotonic sums are dropped above.
			line, err := serializeGaugePoint(
				metric.Name(),
				prefix,
				makeCombinedDimensions(defaultDimensions, dp.Attributes(), staticDimensions),
				dp,
			)

			if err != nil {
				logger.Warn(
					"Error serializing non-monotonic Sum as gauge",
					zap.String("name", metric.Name()),
					zap.String("value-type", dp.ValueType().String()),
					zap.Error(err),
				)
			}

			if line != "" {
				metricLines = append(metricLines, line)
			}
		}
	}

	return metricLines
}

func serializeDeltaCounter(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.NumberDataPoint) (string, error) {
	var valueOpt dtMetric.MetricOption

	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeEmpty:
		return "", fmt.Errorf("unsupported value type none")
	case pmetric.NumberDataPointValueTypeInt:
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleValue())
	default:
		return "", fmt.Errorf("unknown data type")
	}

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		valueOpt,
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}

func serializeCumulativeCounter(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	dm, err := convertTotalCounterToDelta(name, prefix, dims, dp, prev)

	if err != nil {
		return "", err
	}

	if dm == nil {
		return "", nil
	}

	return dm.Serialize()
}

func convertTotalCounterToDelta(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.NumberDataPoint, prevCounters *ttlmap.TTLMap) (*dtMetric.Metric, error) {
	attrPairs := make([]string, 0, dp.Attributes().Len())
	dp.Attributes().Range(func(k string, v pcommon.Value) bool {
		attrPairs = append(attrPairs, k+"="+v.AsString())
		return true
	})
	sort.Strings(attrPairs)
	id := name + strings.Join(attrPairs, ",")

	prevCounter := prevCounters.Get(id)

	if prevCounter == nil {
		prevCounters.Put(id, dp)
		return nil, nil
	}

	oldCount := prevCounter.(pmetric.NumberDataPoint)

	if oldCount.Timestamp().AsTime().After(dp.Timestamp().AsTime()) {
		// current point is older than the previous point
		return nil, nil
	}

	var valueOpt dtMetric.MetricOption

	if dp.ValueType() != oldCount.ValueType() {
		prevCounters.Put(id, dp)
		return nil, fmt.Errorf("expected %s to be type %s but got %s - count reset", name, metricValueTypeToString(oldCount.ValueType()), metricValueTypeToString(dp.ValueType()))
	}

	switch {
	case dp.ValueType() == pmetric.NumberDataPointValueTypeInt:
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntValue() - oldCount.IntValue())
	case dp.ValueType() == pmetric.NumberDataPointValueTypeDouble:
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleValue() - oldCount.DoubleValue())
	default:
		return nil, fmt.Errorf("%s value type %s not supported", name, metricValueTypeToString(dp.ValueType()))
	}

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		valueOpt,
	)

	if err != nil {
		return dm, err
	}

	prevCounters.Put(id, dp)

	return dm, err
}

func metricValueTypeToString(t pmetric.NumberDataPointValueType) string {
	switch t {
	case pmetric.NumberDataPointValueTypeDouble:
		return "MetricValueTypeDouble"
	case pmetric.NumberDataPointValueTypeInt:
		return "MericValueTypeInt"
	case pmetric.NumberDataPointValueTypeEmpty:
		return "MericValueTypeNone"
	default:
		return "MetricValueTypeUnknown"
	}
}
