// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/exphistogram"
)

type ExponentialHistogram struct {
	pmetric.ExponentialHistogramDataPoint
	elasticsearch.MappingHintGetter
	metric           pmetric.Metric
	histogramMapping HistogramMapping
}

func NewExponentialHistogram(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint, hm HistogramMapping) ExponentialHistogram {
	return ExponentialHistogram{
		ExponentialHistogramDataPoint: dp,
		MappingHintGetter:             elasticsearch.NewMappingHintGetter(dp.Attributes()),
		metric:                        metric,
		histogramMapping:              hm,
	}
}

func (dp ExponentialHistogram) Value() (pcommon.Value, error) {
	var counts []int64
	var values []float64
	switch dp.resolvedMapping() {
	case HistogramMappingExponential:
		return exphistogram.ToNativeExponentialHistogram(dp.ExponentialHistogramDataPoint), nil
	case HistogramMappingRaw:
		counts, values = exphistogram.ToRaw(dp.ExponentialHistogramDataPoint)
	case HistogramMappingAggregateMetricDouble:
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	default:
		counts, values = exphistogram.ToTDigest(dp.ExponentialHistogramDataPoint)
	}

	vm := pcommon.NewValueMap()
	m := vm.Map()
	vmCounts := m.PutEmptySlice("counts")
	vmCounts.EnsureCapacity(len(counts))
	for _, c := range counts {
		vmCounts.AppendEmpty().SetInt(c)
	}
	vmValues := m.PutEmptySlice("values")
	vmValues.EnsureCapacity(len(values))
	for _, v := range values {
		vmValues.AppendEmpty().SetDouble(v)
	}

	return vm, nil
}

func (dp ExponentialHistogram) DynamicTemplate(_ pmetric.Metric, mode DynamicTemplateMode) string {
	if mode == DynamicTemplateModeECS {
		if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
			return "summary_metrics"
		}
		return "histogram_metrics"
	}
	// Default mode is otel
	switch dp.resolvedMapping() {
	case HistogramMappingAggregateMetricDouble:
		return "summary"
	case HistogramMappingExponential:
		return "exponential_histogram"
	default:
		return "histogram"
	}
}

func (dp ExponentialHistogram) DocCount() uint64 {
	return dp.Count()
}

func (dp ExponentialHistogram) Metric() pmetric.Metric {
	return dp.metric
}

func (dp ExponentialHistogram) resolvedMapping() HistogramMapping {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		return HistogramMappingAggregateMetricDouble
	}
	if dp.HasMappingHint(elasticsearch.HintHistogramRaw) {
		return HistogramMappingRaw
	}
	return dp.histogramMapping
}
