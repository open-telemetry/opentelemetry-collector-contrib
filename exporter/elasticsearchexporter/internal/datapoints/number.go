// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

var errInvalidNumber = errors.New("invalid number data point")

type Number struct {
	pmetric.NumberDataPoint
	elasticsearch.MappingHintGetter
	metric pmetric.Metric
}

func NewNumber(metric pmetric.Metric, dp pmetric.NumberDataPoint) Number {
	return Number{
		NumberDataPoint:   dp,
		MappingHintGetter: elasticsearch.NewMappingHintGetter(dp.Attributes()),
		metric:            metric,
	}
}

func (dp Number) Value() (pcommon.Value, error) {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		value := dp.DoubleValue()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return pcommon.Value{}, errInvalidNumber
		}
		return pcommon.NewValueDouble(value), nil
	case pmetric.NumberDataPointValueTypeInt:
		return pcommon.NewValueInt(dp.IntValue()), nil
	}
	return pcommon.Value{}, errInvalidNumber
}

func (dp Number) DynamicTemplate(metric pmetric.Metric) string {
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			if metric.Sum().IsMonotonic() {
				return "counter_double"
			}
			return "gauge_double"
		case pmetric.NumberDataPointValueTypeInt:
			if metric.Sum().IsMonotonic() {
				return "counter_long"
			}
			return "gauge_long"
		default:
			return "" // NumberDataPointValueTypeEmpty should already be discarded in numberToValue
		}
	case pmetric.MetricTypeGauge:
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			return "gauge_double"
		case pmetric.NumberDataPointValueTypeInt:
			return "gauge_long"
		default:
			return "" // NumberDataPointValueTypeEmpty should already be discarded in numberToValue
		}
	}
	return ""
}

func (dp Number) DocCount() uint64 {
	return 1
}

func (dp Number) Metric() pmetric.Metric {
	return dp.metric
}
