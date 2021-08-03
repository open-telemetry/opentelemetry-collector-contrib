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

package dynatraceexporter

import (
	"errors"
	"fmt"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func serializeGauge(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint) (string, error) {
	var metricOption dtMetric.MetricOption

	if dp.Type() == pdata.MetricValueTypeInt {
		metricOption = dtMetric.WithIntGaugeValue(dp.IntVal())
	} else if dp.Type() == pdata.MetricValueTypeDouble {
		metricOption = dtMetric.WithFloatGaugeValue(dp.DoubleVal())
	} else {
		return "", nil
	}

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		metricOption,
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}

func serializeSum(name, prefix string, dims dimensions.NormalizedDimensionList, t pdata.AggregationTemporality, dp pdata.NumberDataPoint, prev *ttlmap.TTLMap) (string, error) {
	switch t {
	case pdata.AggregationTemporalityCumulative:
		return serializeCumulativeCounter(name, prefix, dims, dp, prev)
	// for now unspecified is treated as delta
	case pdata.AggregationTemporalityUnspecified:
		fallthrough
	case pdata.AggregationTemporalityDelta:
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

func MetricValueTypeToString(t pdata.MetricValueType) string {
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

func convertTotalCounterToDelta(name, prefix string, dims dimensions.NormalizedDimensionList, dp pdata.NumberDataPoint, prevCounters *ttlmap.TTLMap) (*dtMetric.Metric, error) {
	id := name

	dp.Attributes().Sort().Range(func(k string, v pdata.AttributeValue) bool {
		id += fmt.Sprintf(",%s=%s", k, pdata.AttributeValueToString(v))
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
		return nil, fmt.Errorf("expected %s to be type %s but got %s - count reset", name, MetricValueTypeToString(oldCount.Type()), MetricValueTypeToString(dp.Type()))
	}

	if dp.Type() == pdata.MetricValueTypeInt {
		valueOpt = dtMetric.WithIntCounterValueDelta(dp.IntVal() - oldCount.IntVal())
	} else if dp.Type() == pdata.MetricValueTypeDouble {
		valueOpt = dtMetric.WithFloatCounterValueDelta(dp.DoubleVal() - oldCount.DoubleVal())
	} else {
		return nil, fmt.Errorf("%s value type %s not supported", name, MetricValueTypeToString(dp.Type()))
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

func histMinMax(bounds []float64, counts []uint64) (float64, float64, bool) {
	// Because we do not know the actual min and max, we estimate them based on the min and max non-empty bucket
	minIdx, maxIdx := -1, -1
	for y := 0; y < len(counts); y++ {
		if counts[y] > 0 {
			if minIdx == -1 {
				minIdx = y
			}
			maxIdx = y
		}
	}

	if minIdx == -1 || maxIdx == -1 {
		return 0, 0, false
	}

	var min, max float64

	// Use lower bound for min unless it is the first bucket, then use upper
	if minIdx == 0 {
		min = bounds[minIdx]
	} else {
		min = bounds[minIdx-1]
	}

	// Use upper bound for max unless it is the last bucket, then use lower
	if maxIdx == len(counts)-1 {
		max = bounds[maxIdx-1]
	} else {
		max = bounds[maxIdx]
	}

	return min, max, true
}

func serializeHistogram(name, prefix string, dims dimensions.NormalizedDimensionList, t pdata.AggregationTemporality, dp pdata.HistogramDataPoint) (string, error) {
	if t == pdata.AggregationTemporalityCumulative {
		// convert to delta histogram
		// skip first point because there is nothing to calculate a delta from
		// what if bucket bounds change
		// TTL for cumulative histograms
		// reset detection? if cumulative and count decreases, the process probably reset
		return "", errors.New("cumulative histograms not supported")
	}

	min, max, nonEmpty := histMinMax(dp.ExplicitBounds(), dp.BucketCounts())

	if !nonEmpty {
		return "", nil
	}

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		dtMetric.WithFloatSummaryValue(
			min,
			max,
			dp.Sum(),
			int64(dp.Count()),
		),
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}
