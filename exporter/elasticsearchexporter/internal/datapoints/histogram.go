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
	raw := dp.HasMappingHint(elasticsearch.HintHistogramRaw)
	return histogramToValue(dp.HistogramDataPoint, dp.metric, raw)
}

func (dp Histogram) DynamicTemplate(_ pmetric.Metric, mode DynamicTemplateMode) string {
	if mode == DynamicTemplateModeECS {
		if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
			return "summary_metrics"
		}
		return "histogram_metrics"
	}

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

func histogramToValue(dp pmetric.HistogramDataPoint, metric pmetric.Metric, raw bool) (pcommon.Value, error) {
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
		if raw && i == explicitBounds.Len() {
			// In raw mode, the overflow bucket would have the same value
			// as the last real bucket. Merge the overflow count into the
			// last real bucket to avoid duplicate values, which violates
			// ES histogram's strictly increasing values requirement.
			lastIdx := counts.Len() - 1
			if lastIdx >= 0 && count > 0 {
				counts.At(lastIdx).SetInt(counts.At(lastIdx).Int() + safeUint64ToInt64(count))
			}
			break
		}

		var value float64
		if raw {
			value = rawBucketValue(explicitBounds, i)
		} else {
			value = midpointBucketValue(explicitBounds, i)
		}

		counts.AppendEmpty().SetInt(safeUint64ToInt64(count))
		values.AppendEmpty().SetDouble(value)
	}

	return vm, nil
}

// midpointBucketValue returns the midpoint centroid for bucket at index i.
// Caller must ensure len(bucketCounts) == len(explicitBounds) + 1.
func midpointBucketValue(explicitBounds pcommon.Float64Slice, i int) float64 {
	switch {
	// (-infinity, explicit_bounds[i]]
	case i == 0:
		v := explicitBounds.At(i)
		if v > 0 {
			v /= 2
		}
		return v

	// (explicit_bounds[i-1], +infinity)
	case i >= explicitBounds.Len():
		return explicitBounds.At(i - 1)

	// [explicit_bounds[i-1], explicit_bounds[i])
	default:
		return explicitBounds.At(i-1) + (explicitBounds.At(i)-explicitBounds.At(i-1))/2.0
	}
}

// rawBucketValue returns the explicit bound for bucket at index i without
// any midpoint approximation.
func rawBucketValue(explicitBounds pcommon.Float64Slice, i int) float64 {
	return explicitBounds.At(i)
}
