// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"

import (
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
	if metric.Type() == pmetric.MetricTypeGauge {
		dataPoints := metric.Gauge().DataPoints()
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
	return 0
}

// generateMetrics creates a new metric based on the given rule and add it to the Resource Metric.
// The value for newly calculated metrics is always a floting point number and the dataType is set
// as MetricTypeDoubleGauge.
func generateMetrics(rm pmetric.ResourceMetrics, operand2 float64, rule internalRule, logger *zap.Logger) {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			if metric.Name() == rule.metric1 {
				newMetric := appendMetric(ilm, rule.name, rule.unit)
				newMetric.SetEmptyGauge()
				addDoubleGaugeDataPoints(metric, newMetric, operand2, rule.operation, logger)
			}
		}
	}
}

func addDoubleGaugeDataPoints(from pmetric.Metric, to pmetric.Metric, operand2 float64, operation string, logger *zap.Logger) {
	dataPoints := from.Gauge().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		fromDataPoint := dataPoints.At(i)
		var operand1 float64
		switch fromDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			operand1 = fromDataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			operand1 = float64(fromDataPoint.IntValue())
		}

		neweDoubleDataPoint := to.Gauge().DataPoints().AppendEmpty()
		fromDataPoint.CopyTo(neweDoubleDataPoint)
		value := calculateValue(operand1, operand2, operation, logger, to.Name())
		neweDoubleDataPoint.SetDoubleValue(value)
	}
}

func appendMetric(ilm pmetric.ScopeMetrics, name, unit string) pmetric.Metric {
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}

func calculateValue(operand1 float64, operand2 float64, operation string, logger *zap.Logger, metricName string) float64 {
	switch operation {
	case string(add):
		return operand1 + operand2
	case string(subtract):
		return operand1 - operand2
	case string(multiply):
		return operand1 * operand2
	case string(divide):
		if operand2 == 0 {
			logger.Debug("Divide by zero was attempted while calculating metric", zap.String("metric_name", metricName))
			return 0
		}
		return operand1 / operand2
	case string(percent):
		if operand2 == 0 {
			logger.Debug("Divide by zero was attempted while calculating metric", zap.String("metric_name", metricName))
			return 0
		}
		return (operand1 / operand2) * 100
	}
	return 0
}
