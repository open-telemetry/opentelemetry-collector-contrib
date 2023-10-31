// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"fmt"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func serializeGaugePoint(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.NumberDataPoint) (string, error) {
	var metricOption dtMetric.MetricOption

	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeEmpty:
		return "", fmt.Errorf("unsupported value type none")
	case pmetric.NumberDataPointValueTypeInt:
		metricOption = dtMetric.WithIntGaugeValue(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		metricOption = dtMetric.WithFloatGaugeValue(dp.DoubleValue())
	default:
		return "", fmt.Errorf("unknown data type")
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

func serializeGauge(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions dimensions.NormalizedDimensionList, staticDimensions dimensions.NormalizedDimensionList, metricLines []string) []string {
	points := metric.Gauge().DataPoints()

	for i := 0; i < points.Len(); i++ {
		dp := points.At(i)

		line, err := serializeGaugePoint(
			metric.Name(),
			prefix,
			makeCombinedDimensions(defaultDimensions, dp.Attributes(), staticDimensions),
			dp,
		)

		if err != nil {
			logger.Warn(
				"Error serializing gauge data point",
				zap.String("name", metric.Name()),
				zap.String("value-type", dp.ValueType().String()),
				zap.Error(err),
			)
		}

		if line != "" {
			metricLines = append(metricLines, line)
		}
	}
	return metricLines
}
