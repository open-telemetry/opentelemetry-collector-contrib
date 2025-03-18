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

// generateCalculatedMetrics creates a new metric based on the given rule and adds it to the scope metric.
// The value for newly calculated metrics is always a floating point number.
// Note: This method assumes the matchAttributes feature flag is enabled.
func generateCalculatedMetrics(rm pmetric.ResourceMetrics, metric2 pmetric.Metric, rule internalRule, logger *zap.Logger) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)

			if metric.Name() == rule.metric1 {
				newMetric := generateMetricFromMatchingAttributes(metric, metric2, rule, logger)
				appendNewMetric(ilm, newMetric, rule.name, rule.unit)
			}
		}
	}
}

// Calculates a new metric based on the calculation-type rule specified. New data points will be generated for each
// calculation of the input metrics where overlapping attributes have matching values.
func generateMetricFromMatchingAttributes(metric1 pmetric.Metric, metric2 pmetric.Metric, rule internalRule, logger *zap.Logger) pmetric.Metric {
	var metric1DataPoints pmetric.NumberDataPointSlice
	var toDataPoints pmetric.NumberDataPointSlice
	to := pmetric.NewMetric()

	// Setup to metric and get metric1 data points
	switch metricType := metric1.Type(); metricType {
	case pmetric.MetricTypeGauge:
		to.SetEmptyGauge()
		metric1DataPoints = metric1.Gauge().DataPoints()
		toDataPoints = to.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		to.SetEmptySum()
		metric1DataPoints = metric1.Sum().DataPoints()
		toDataPoints = to.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", metric1.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	// Get metric2 data points
	var metric2DataPoints pmetric.NumberDataPointSlice
	switch metricType := metric2.Type(); metricType {
	case pmetric.MetricTypeGauge:
		metric2DataPoints = metric2.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		metric2DataPoints = metric2.Sum().DataPoints()
	default:
		logger.Debug(fmt.Sprintf("Calculations are only supported on gauge or sum metric types. Given metric '%s' is of type `%s`", metric2.Name(), metricType.String()))
		return pmetric.NewMetric()
	}

	for i := 0; i < metric1DataPoints.Len(); i++ {
		metric1DP := metric1DataPoints.At(i)

		for j := 0; j < metric2DataPoints.Len(); j++ {
			metric2DP := metric2DataPoints.At(j)
			if dataPointAttributesMatch(metric1DP, metric2DP) {
				val, err := calculateValue(dataPointValue(metric1DP), dataPointValue(metric2DP), rule.operation, rule.name)

				if err != nil {
					logger.Debug(err.Error())
				} else {
					newDP := toDataPoints.AppendEmpty()
					metric1DP.CopyTo(newDP)
					newDP.SetDoubleValue(val)

					for k, v := range metric2DP.Attributes().All() {
						v.CopyTo(newDP.Attributes().PutEmpty(k))
						// Always return true to ensure iteration over all attributes
					}
				}
			}
		}
	}

	return to
}

func dataPointValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	default:
		return 0
	}
}

func dataPointAttributesMatch(dp1, dp2 pmetric.NumberDataPoint) bool {
	attributesMatch := true
	for key, dp1Val := range dp1.Attributes().All() {
		dp1Val.Type()
		if dp2Val, keyExists := dp2.Attributes().Get(key); keyExists && dp1Val.AsRaw() != dp2Val.AsRaw() {
			return false
		}
	}

	return attributesMatch
}

// generateScalarMetrics creates a new metric based on a scalar type rule and adds it to the scope metric.
// The value for newly calculated metrics is always a floating point number.
func generateScalarMetrics(rm pmetric.ResourceMetrics, operand2 float64, rule internalRule, logger *zap.Logger) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			if metric.Name() == rule.metric1 {
				newMetric := generateMetricFromOperand(metric, operand2, rule.operation, logger)
				appendNewMetric(ilm, newMetric, rule.name, rule.unit)
			}
		}
	}
}

func generateMetricFromOperand(from pmetric.Metric, operand2 float64, operation string, logger *zap.Logger) pmetric.Metric {
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

// Append the new metric to the scope metrics. This will only append the new metric if it
// has data points.
func appendNewMetric(ilm pmetric.ScopeMetrics, newMetric pmetric.Metric, name, unit string) {
	dataPointCount := 0
	switch newMetric.Type() {
	case pmetric.MetricTypeSum:
		dataPointCount = newMetric.Sum().DataPoints().Len()
	case pmetric.MetricTypeGauge:
		dataPointCount = newMetric.Gauge().DataPoints().Len()
	}

	// Only create a new metric if valid data points were calculated successfully
	if dataPointCount > 0 {
		metric := ilm.Metrics().AppendEmpty()
		newMetric.MoveTo(metric)

		metric.SetUnit(unit)
		metric.SetName(name)
	}
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
