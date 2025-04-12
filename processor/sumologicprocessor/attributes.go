// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// This file contains some common functionalities for subprocessors that modify attributes (represented by pcommon.Map)

type attributesProcessor interface {
	processAttributes(pcommon.Map) error
}

func processMetricLevelAttributes(proc attributesProcessor, metric pmetric.Metric) error {
	switch metric.Type() {
	case pmetric.MetricTypeEmpty:
		return nil

	case pmetric.MetricTypeSum:
		dp := metric.Sum().DataPoints()
		for i := 0; i < dp.Len(); i++ {
			err := proc.processAttributes(dp.At(i).Attributes())
			if err != nil {
				return err
			}
		}
		return nil

	case pmetric.MetricTypeGauge:
		dp := metric.Gauge().DataPoints()
		for i := 0; i < dp.Len(); i++ {
			err := proc.processAttributes(dp.At(i).Attributes())
			if err != nil {
				return err
			}
		}
		return nil

	case pmetric.MetricTypeHistogram:
		dp := metric.Histogram().DataPoints()
		for i := 0; i < dp.Len(); i++ {
			err := proc.processAttributes(dp.At(i).Attributes())
			if err != nil {
				return err
			}
		}
		return nil

	case pmetric.MetricTypeExponentialHistogram:
		dp := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dp.Len(); i++ {
			err := proc.processAttributes(dp.At(i).Attributes())
			if err != nil {
				return err
			}
		}
		return nil

	case pmetric.MetricTypeSummary:
		dp := metric.Summary().DataPoints()
		for i := 0; i < dp.Len(); i++ {
			err := proc.processAttributes(dp.At(i).Attributes())
			if err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("unknown metric type: %s", metric.Type().String())
}

func mapToPcommonMap(m map[string]pcommon.Value) pcommon.Map {
	attrs := pcommon.NewMap()
	for k, v := range m {
		v.CopyTo(attrs.PutEmpty(k))
	}

	return attrs
}

func mapToPcommonValue(m map[string]pcommon.Value) pcommon.Value {
	attrs := pcommon.NewValueMap()
	for k, v := range m {
		v.CopyTo(attrs.Map().PutEmpty(k))
	}

	return attrs
}
