// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type Summary struct {
	pmetric.SummaryDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func NewSummary(metric pmetric.Metric, dp pmetric.SummaryDataPoint) Summary {
	return Summary{
		SummaryDataPoint:  dp,
		MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()),
		metric:            metric,
	}
}

func (dp Summary) Value() (pcommon.Value, error) {
	// TODO: Add support for quantiles
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
	vm := pcommon.NewValueMap()
	m := vm.Map()
	m.PutDouble("sum", dp.Sum())
	m.PutInt("value_count", safeUint64ToInt64(dp.Count()))
	return vm, nil
}

func (dp Summary) DynamicTemplate(_ pmetric.Metric) string {
	return "summary"
}

func (dp Summary) DocCount() uint64 {
	return dp.Count()
}

func (dp Summary) Metric() pmetric.Metric {
	return dp.metric
}
