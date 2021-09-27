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

package serialization

import (
	"fmt"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func serializeSum(name, prefix string, dims dimensions.NormalizedDimensionList, t pdata.MetricAggregationTemporality, dp pdata.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	switch t {
	case pdata.MetricAggregationTemporalityCumulative:
		return serializeCumulativeCounter(name, prefix, dims, dp, prev)
	// for now unspecified is treated as delta
	case pdata.MetricAggregationTemporalityUnspecified:
		fallthrough
	case pdata.MetricAggregationTemporalityDelta:
		return serializeDeltaCounter(name, prefix, dims, dp)
	}

	return "", nil
}

func serializeDeltaCounter(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint) (string, error) {
	var valueOpt dtMetric.MetricOption

	if dp.Type() == pdata.MetricValueTypeInt {
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal())
	} else if dp.Type() == pdata.MetricValueTypeDouble {
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal())
	} else {
		return "", nil
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

func serializeCumulativeCounter(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	dm, err := convertTotalCounterToDelta(name, prefix, dims, dp, prev)

	if err != nil {
		return "", err
	}

	if dm == nil {
		return "", nil
	}

	return dm.Serialize()
}

func convertTotalCounterToDelta(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint, prevCounters *ttlmap.TTLMap) (*dtMetric.Metric, error) {
	id := name

	dp.Attributes().Sort().Range(func(k string, v pdata.AttributeValue) bool {
		id += fmt.Sprintf(",%s=%s", k, v.AsString())
		return true
	})

	prevCounter := prevCounters.Get(id)

	if prevCounter == nil {
		prevCounters.Put(id, dp)
		return nil, nil
	}

	oldCount := prevCounter.(pdata.NumberDataPoint)

	if oldCount.Timestamp().AsTime().After(dp.Timestamp().AsTime()) {
		// current point is older than the previous point
		return nil, nil
	}

	var valueOpt dtMetric.MetricOption

	if dp.Type() != oldCount.Type() {
		prevCounters.Put(id, dp)
		return nil, fmt.Errorf("expected %s to be type %s but got %s - count reset", name, metricValueTypeToString(oldCount.Type()), metricValueTypeToString(dp.Type()))
	}

	if dp.Type() == pdata.MetricValueTypeInt {
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal() - oldCount.IntVal())
	} else if dp.Type() == pdata.MetricValueTypeDouble {
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal() - oldCount.DoubleVal())
	} else {
		return nil, fmt.Errorf("%s value type %s not supported", name, metricValueTypeToString(dp.Type()))
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

func metricValueTypeToString(t pdata.MetricValueType) string {
	switch t {
	case pdata.MetricValueTypeDouble:
		return "MetricValueTypeDouble"
	case pdata.MetricValueTypeInt:
		return "MericValueTypeInt"
	case pdata.MetricValueTypeNone:
		return "MericValueTypeNone"
	default:
		return "MetricValueTypeUnknown"
	}
}
