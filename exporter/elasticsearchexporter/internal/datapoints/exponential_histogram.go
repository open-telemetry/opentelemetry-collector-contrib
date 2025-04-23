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
	metric pmetric.Metric
}

func NewExponentialHistogram(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) ExponentialHistogram {
	return ExponentialHistogram{
		ExponentialHistogramDataPoint: dp,
		MappingHintGetter:             elasticsearch.NewMappingHintGetter(dp.Attributes()),
		metric:                        metric,
	}
}

func (dp ExponentialHistogram) Value() (pcommon.Value, error) {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		vm := pcommon.NewValueMap()
		m := vm.Map()
		m.PutDouble("sum", dp.Sum())
		m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
		return vm, nil
	}

	counts, values := exphistogram.ToTDigest(dp.ExponentialHistogramDataPoint)

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

func (dp ExponentialHistogram) DynamicTemplate(_ pmetric.Metric) string {
	if dp.HasMappingHint(elasticsearch.HintAggregateMetricDouble) {
		return "summary"
	}
	return "histogram"
}

func (dp ExponentialHistogram) DocCount() uint64 {
	return dp.Count()
}

func (dp ExponentialHistogram) Metric() pmetric.Metric {
	return dp.metric
}
