// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricsgenerationprocessor

import (
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func getNameToMetricMap(rm pdata.ResourceMetrics) map[string]pdata.Metric {
	ilms := rm.InstrumentationLibraryMetrics()
	metricMap := make(map[string]pdata.Metric)

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
func getMetricValue(metric pdata.Metric) float64 {
	if metric.DataType() == pdata.MetricDataTypeGauge {
		dataPoints := metric.Gauge().DataPoints()
		if dataPoints.Len() > 0 {
			switch dataPoints.At(0).Type() {
			case pdata.MetricValueTypeDouble:
				return dataPoints.At(0).DoubleVal()
			case pdata.MetricValueTypeInt:
				return float64(dataPoints.At(0).IntVal())
			}
		}
		return 0
	}
	return 0
}

// generateMetrics creates a new metric based on the given rule and add it to the Resource Metric.
// The value for newly calculated metrics is always a floting point number and the dataType is set
// as MetricDataTypeDoubleGauge.
func generateMetrics(rm pdata.ResourceMetrics, operand2 float64, rule internalRule, logger *zap.Logger) {
	ilms := rm.InstrumentationLibraryMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		metricSlice := ilm.Metrics()
		for j := 0; j < metricSlice.Len(); j++ {
			metric := metricSlice.At(j)
			if metric.Name() == rule.metric1 {
				newMetric := appendMetric(ilm, rule.name, rule.unit)
				newMetric.SetDataType(pdata.MetricDataTypeGauge)
				addDoubleGaugeDataPoints(metric, newMetric, operand2, rule.operation, logger)
			}
		}
	}
}

func addDoubleGaugeDataPoints(from pdata.Metric, to pdata.Metric, operand2 float64, operation string, logger *zap.Logger) {
	dataPoints := from.Gauge().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		fromDataPoint := dataPoints.At(i)
		var operand1 float64
		switch fromDataPoint.Type() {
		case pdata.MetricValueTypeDouble:
			operand1 = fromDataPoint.DoubleVal()
		case pdata.MetricValueTypeInt:
			operand1 = float64(fromDataPoint.IntVal())
		}

		neweDoubleDataPoint := to.Gauge().DataPoints().AppendEmpty()
		fromDataPoint.CopyTo(neweDoubleDataPoint)
		value := calculateValue(operand1, operand2, operation, logger, to.Name())
		neweDoubleDataPoint.SetDoubleVal(value)
	}
}

func appendMetric(ilm pdata.InstrumentationLibraryMetrics, name, unit string) pdata.Metric {
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
