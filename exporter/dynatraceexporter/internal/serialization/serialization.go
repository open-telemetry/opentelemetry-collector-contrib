// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"fmt"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func SerializeMetric(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions, staticDimensions dimensions.NormalizedDimensionList, prev *ttlmap.TTLMap) ([]string, error) {
	var metricLines []string

	ce := logger.Check(zap.DebugLevel, "SerializeMetric")
	var points int
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		metricLines = serializeGauge(logger, prefix, metric, defaultDimensions, staticDimensions, metricLines)
	case pmetric.MetricTypeSum:
		metricLines = serializeSum(logger, prefix, metric, defaultDimensions, staticDimensions, prev, metricLines)
	case pmetric.MetricTypeHistogram:
		metricLines = serializeHistogram(logger, prefix, metric, defaultDimensions, staticDimensions, metricLines)
	case pmetric.MetricTypeEmpty:
		fallthrough
	case pmetric.MetricTypeExponentialHistogram:
		fallthrough
	case pmetric.MetricTypeSummary:
		fallthrough
	default:
		return nil, fmt.Errorf("metric type %s unsupported", metric.Type().String())
	}

	if ce != nil {
		ce.Write(zap.String("DataType", metric.Type().String()), zap.Int("points", points))
	}

	return metricLines, nil
}

func makeCombinedDimensions(defaultDimensions dimensions.NormalizedDimensionList, dataPointAttributes pcommon.Map, staticDimensions dimensions.NormalizedDimensionList) dimensions.NormalizedDimensionList {
	dimsFromAttributes := make([]dimensions.Dimension, 0, dataPointAttributes.Len())

	dataPointAttributes.Range(func(k string, v pcommon.Value) bool {
		dimsFromAttributes = append(dimsFromAttributes, dimensions.NewDimension(k, v.AsString()))
		return true
	})
	return dimensions.MergeLists(
		defaultDimensions,
		dimensions.NewNormalizedDimensionList(dimsFromAttributes...),
		staticDimensions,
	)
}
