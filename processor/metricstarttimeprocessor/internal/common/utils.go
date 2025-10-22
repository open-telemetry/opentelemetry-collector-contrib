package common

import "go.opentelemetry.io/collector/pdata/pmetric"

func HasStartTimeSet(metric pmetric.Metric) bool {
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		if dataPoints.Len() > 0 {
			return dataPoints.At(0).StartTimestamp() != 0
		}
	case pmetric.MetricTypeHistogram:
		dataPoints := metric.Histogram().DataPoints()
		if dataPoints.Len() > 0 {
			return dataPoints.At(0).StartTimestamp() != 0
		}
	case pmetric.MetricTypeExponentialHistogram:
		dataPoints := metric.ExponentialHistogram().DataPoints()
		if dataPoints.Len() > 0 {
			return dataPoints.At(0).StartTimestamp() != 0
		}
	case pmetric.MetricTypeSummary:
		dataPoints := metric.Summary().DataPoints()
		if dataPoints.Len() > 0 {
			return dataPoints.At(0).StartTimestamp() != 0
		}
	}
	return false
}
