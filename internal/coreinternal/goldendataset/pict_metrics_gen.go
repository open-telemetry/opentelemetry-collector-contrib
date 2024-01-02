// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// GenerateMetrics takes the filename of a PICT-generated file, walks through all of the rows in the PICT
// file and for each row, generates a MetricData object, collecting them and returning them to the caller.
func GenerateMetrics(metricPairsFile string) ([]pmetric.Metrics, error) {
	pictData, err := loadPictOutputFile(metricPairsFile)
	if err != nil {
		return nil, err
	}
	out := make([]pmetric.Metrics, 0, len(pictData))
	for i, values := range pictData {
		if i == 0 {
			continue
		}
		metricInputs := PICTMetricInputs{
			NumPtsPerMetric: PICTNumPtsPerMetric(values[0]),
			MetricType:      PICTMetricType(values[1]),
			NumPtLabels:     PICTNumPtLabels(values[2]),
		}
		cfg := pictToCfg(metricInputs)
		cfg.MetricNamePrefix = fmt.Sprintf("pict_%d_", i)
		md := MetricsFromCfg(cfg)
		out = append(out, md)
	}
	return out, nil
}

func pictToCfg(inputs PICTMetricInputs) MetricsCfg {
	cfg := DefaultCfg()
	switch inputs.NumResourceAttrs {
	case AttrsNone:
		cfg.NumResourceAttrs = 0
	case AttrsOne:
		cfg.NumResourceAttrs = 1
	case AttrsTwo:
		cfg.NumResourceAttrs = 2
	}

	switch inputs.NumPtsPerMetric {
	case NumPtsPerMetricOne:
		cfg.NumPtsPerMetric = 1
	case NumPtsPerMetricMany:
		cfg.NumPtsPerMetric = 16
	}

	switch inputs.MetricType {
	case MetricTypeIntGauge:
		cfg.MetricDescriptorType = pmetric.MetricTypeGauge
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeInt
	case MetricTypeMonotonicIntSum:
		cfg.MetricDescriptorType = pmetric.MetricTypeSum
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeInt
		cfg.IsMonotonicSum = true
	case MetricTypeNonMonotonicIntSum:
		cfg.MetricDescriptorType = pmetric.MetricTypeSum
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeInt
		cfg.IsMonotonicSum = false
	case MetricTypeDoubleGauge:
		cfg.MetricDescriptorType = pmetric.MetricTypeGauge
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
	case MetricTypeMonotonicDoubleSum:
		cfg.MetricDescriptorType = pmetric.MetricTypeSum
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
		cfg.IsMonotonicSum = true
	case MetricTypeNonMonotonicDoubleSum:
		cfg.MetricDescriptorType = pmetric.MetricTypeSum
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeDouble
		cfg.IsMonotonicSum = false
	case MetricTypeDoubleExemplarsHistogram:
		cfg.MetricDescriptorType = pmetric.MetricTypeHistogram
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeEmpty
	case MetricTypeIntExemplarsHistogram:
		cfg.MetricDescriptorType = pmetric.MetricTypeHistogram
		cfg.MetricValueType = pmetric.NumberDataPointValueTypeEmpty
	default:
		panic("Should not happen, unsupported type " + string(inputs.MetricType))
	}

	switch inputs.NumPtLabels {
	case LabelsNone:
		cfg.NumPtLabels = 0
	case LabelsOne:
		cfg.NumPtLabels = 1
	case LabelsMany:
		cfg.NumPtLabels = 16
	}
	return cfg
}
