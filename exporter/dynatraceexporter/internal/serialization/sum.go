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

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func serializeSum(name, prefix string, dims dimensions.NormalizedDimensionList, t pmetric.MetricAggregationTemporality, dp pmetric.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
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

func serializeDeltaCounter(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.NumberDataPoint) (string, error) {
	var valueOpt dtMetric.MetricOption

	switch dp.ValueType() {
	case pmetric.MetricValueTypeNone:
		return "", fmt.Errorf("unsupported value type none")
	case pmetric.MetricValueTypeInt:
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal())
	case pmetric.MetricValueTypeDouble:
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

	if dp.ValueType() == pmetric.MetricValueTypeInt {
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal() - oldCount.IntVal())
	} else if dp.ValueType() == pmetric.MetricValueTypeDouble {
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal() - oldCount.DoubleVal())
	} else {
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

func metricValueTypeToString(t pmetric.MetricValueType) string {
	switch t {
	case pmetric.MetricValueTypeDouble:
		return "MetricValueTypeDouble"
	case pmetric.MetricValueTypeInt:
		return "MericValueTypeInt"
	case pmetric.MetricValueTypeNone:
		return "MericValueTypeNone"
	default:
		return "MetricValueTypeUnknown"
	}
}
