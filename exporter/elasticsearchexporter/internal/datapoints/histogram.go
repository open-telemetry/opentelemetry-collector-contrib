// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type Histogram struct {
	pmetric.HistogramDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func NewHistogram(metric pmetric.Metric, dp pmetric.HistogramDataPoint) Histogram {
	return Histogram{
		HistogramDataPoint: dp,
		MappingHintGetter:  elasticsearch.NewMappingHintGetter(dp.Attributes()),
		metric:             metric,
	}
}

func (dp Histogram) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	}
	return histogramToValue(dp.HistogramDataPoint, dp.metric)
}

func (dp Histogram) DynamicTemplate(_ pmetric.Metric) string {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp Histogram) DocCount() uint64 {
	return dp.Count()
}

func (dp Histogram) Metric() pmetric.Metric {
	return dp.metric
}

func histogramToValue(dp pmetric.HistogramDataPoint, metric pmetric.Metric) (pcommon.Value, error) {
	bucketCounts := dp.BucketCounts()
	explicitBounds := dp.ExplicitBounds()
	if bucketCounts.Len() > 0 && bucketCounts.Len() != explicitBounds.Len()+1 {
		return pcommon.Value{}, fmt.Errorf("invalid histogram data point %q", metric.Name())
	}

	vm := pcommon.NewValueMap()
	m := vm.Map()
	counts := m.PutEmptySlice("counts")
	values := m.PutEmptySlice("values")

	if explicitBounds.Len() == 0 {
		// It is possible for explicit bounds to be nil. In this case create
		// a bucket using the count and sum which are required to be present.
		// See https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram
		if dp.Count() > 0 {
			counts.AppendEmpty().SetInt(safeUint64ToInt64(dp.Count()))
			values.AppendEmpty().SetDouble(dp.Sum() / float64(dp.Count()))
		}

		return vm, nil
	}

	counts.EnsureCapacity(bucketCounts.Len())
	values.EnsureCapacity(bucketCounts.Len())
	for i, count := range bucketCounts.All() {
		if count == 0 {
			continue
		}

		var value float64
		switch i {
		// (-infinity, explicit_bounds[i]]
		case 0:
			value = explicitBounds.At(i)
			if value > 0 {
				value /= 2
			}

		// (explicit_bounds[i], +infinity)
		case bucketCounts.Len() - 1:
			value = explicitBounds.At(i - 1)

		// [explicit_bounds[i-1], explicit_bounds[i])
		default:
			// Use the midpoint between the boundaries.
			value = explicitBounds.At(i-1) + (explicitBounds.At(i)-explicitBounds.At(i-1))/2.0
		}

		counts.AppendEmpty().SetInt(safeUint64ToInt64(count))
		values.AppendEmpty().SetDouble(value)
	}

	return vm, nil
}
