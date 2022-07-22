// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"fmt"

	"go.uber.org/zap"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func serializeSumPoint(name, prefix string, dims dimensions.NormalizedDimensionList, t pmetric.MetricAggregationTemporality, dp pmetric.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	switch t {
	case pmetric.MetricAggregationTemporalityCumulative:
		return serializeCumulativeCounter(name, prefix, dims, dp, prev)
	// for now unspecified is treated as delta
	case pmetric.MetricAggregationTemporalityUnspecified:
		fallthrough
	case pmetric.MetricAggregationTemporalityDelta:
		return serializeDeltaCounter(name, prefix, dims, dp)
	}

	return "", nil
}

func serializeSum(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions dimensions.NormalizedDimensionList, staticDimensions dimensions.NormalizedDimensionList, prev *ttlmap.TTLMap, metricLines []string) []string {
	sum := metric.Sum()

	if !sum.IsMonotonic() && sum.AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
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
	case pmetric.NumberDataPointValueTypeNone:
		return "", fmt.Errorf("unsupported value type none")
	case pmetric.NumberDataPointValueTypeInt:
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal())
	case pmetric.NumberDataPointValueTypeDouble:
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal())
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
	id := name

	dp.Attributes().Sort().Range(func(k string, v pcommon.Value) bool {
		id += fmt.Sprintf(",%s=%s", k, v.AsString())
		return true
	})

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
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal() - oldCount.IntVal())
	case dp.ValueType() == pmetric.NumberDataPointValueTypeDouble:
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal() - oldCount.DoubleVal())
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
	case pmetric.NumberDataPointValueTypeNone:
		return "MericValueTypeNone"
	default:
		return "MetricValueTypeUnknown"
	}
}
