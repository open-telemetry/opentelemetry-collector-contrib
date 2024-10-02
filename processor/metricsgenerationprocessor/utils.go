// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func getNameToMetricMap(rm pmetric.ResourceMetrics) map[string]pmetric.Metric {
	ilms := rm.ScopeMetrics()
	metricMap := make(map[string]pmetric.Metric)

	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			metricMap[metric.Name()] = metric
		}
	}
	return metricMap
}

// getMetricValue returns the value of the first data point from the given metric.
func getMetricValue(metric pmetric.Metric) float64 {
	var dataPoints pmetric.NumberDataPointSlice

	switch metricType := metric.Type(); metricType {
	case pmetric.MetricTypeGauge:
		dataPoints = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPoints = metric.Sum().DataPoints()
	default:
		return 0
	}

	if dataPoints.Len() > 0 {
		switch dataPoints.At(0).ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			return dataPoints.At(0).DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			return float64(dataPoints.At(0).IntValue())
		}
	}

	return 0
}

// generateMetrics creates a new metric based on the given rule and add it to the Resource Metric.
// The value for newly calculated metrics is always a floating point number.
func generateMetrics(rm pmetric.ResourceMetrics, operand2 float64, rule internalRule, logger *zap.Logger) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			if metric.Name() == rule.metric1 {
				newMetric := generateMetric(metric, operand2, rule.operation, logger)

				dataPointCount := 0
				switch newMetric.Type() {
				case pmetric.MetricTypeSum:
					dataPointCount = newMetric.Sum().DataPoints().Len()
				case pmetric.MetricTypeGauge:
					dataPointCount = newMetric.Gauge().DataPoints().Len()
				}

				// Only create a new metric if valid data points were calculated successfully
				if dataPointCount > 0 {
					appendMetric(ilm, newMetric, rule.name, rule.unit)
				}
			}
		}
	}
}

func generateMetric(from pmetric.Metric, operand2 float64, operation string, logger *zap.Logger) pmetric.Metric {
	var dataPoints pmetric.NumberDataPointSlice
	to := pmetric.NewMetric()

	switch metricType := from.Type(); metricType {
	case pmetric.MetricTypeGauge:
		to.SetEmptyGauge()
		dataPoints = from.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		to.SetEmptySum()
		dataPoints = from.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", from.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	for i := 0; i < dataPoints.Len(); i++ {
		fromDataPoint := dataPoints.At(i)
		var operand1 float64
		switch fromDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			operand1 = fromDataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			operand1 = float64(fromDataPoint.IntValue())
		}
		value, err := calculateValue(operand1, operand2, operation, to.Name())

		// Only add a new data point if it was a valid operation
		if err != nil {
			logger.Debug(err.Error())
		} else {
			var newDoubleDataPoint pmetric.NumberDataPoint
			switch to.Type() {
			case pmetric.MetricTypeGauge:
				newDoubleDataPoint = to.Gauge().DataPoints().AppendEmpty()
			case pmetric.MetricTypeSum:
				newDoubleDataPoint = to.Sum().DataPoints().AppendEmpty()
			}

			fromDataPoint.CopyTo(newDoubleDataPoint)
			newDoubleDataPoint.SetDoubleValue(value)
		}
	}

	return to
}

// Append the scope metrics with the new metric
func appendMetric(ilm pmetric.ScopeMetrics, newMetric pmetric.Metric, name, unit string) pmetric.Metric {
	metric := ilm.Metrics().AppendEmpty()
	newMetric.MoveTo(metric)

	metric.SetUnit(unit)
	metric.SetName(name)

	return metric
}

func calculateValue(operand1 float64, operand2 float64, operation string, metricName string) (float64, error) {
	switch operation {
	case string(add):
		return operand1 + operand2, nil
	case string(subtract):
		return operand1 - operand2, nil
	case string(multiply):
		return operand1 * operand2, nil
	case string(divide):
		if operand2 == 0 {
			return 0, fmt.Errorf("Divide by zero was attempted while calculating metric: %s", metricName)
		}
		return operand1 / operand2, nil
	case string(percent):
		if operand2 == 0 {
			return 0, fmt.Errorf("Divide by zero was attempted while calculating metric: %s", metricName)
		}
		return (operand1 / operand2) * 100, nil
	default:
		return 0, fmt.Errorf("Invalid operation option was specified: %s", operation)
	}
}
