package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func CopyTimeStamps(expected, actual pmetric.Metrics) {
	expectedRms := expected.ResourceMetrics()
	actualRms := actual.ResourceMetrics()
	pathHash := make(map[[16]byte]pmetric.MetricSlice)
	for i := 0; i < expectedRms.Len(); i++ {
		pathHashKey := pdatautil.MapHash(expectedRms.At(i).Resource().Attributes())
		expectedIms := expectedRms.At(i).ScopeMetrics()
		for j := 0; j < expectedIms.Len(); j++ {
			tempMetrics := expectedIms.At(j).Metrics()
			pathHash[pathHashKey] = tempMetrics
		}
	}

	for key, value := range pathHash {
		for i := 0; i < actualRms.Len(); i++ {
			if key == pdatautil.MapHash(actualRms.At(i).Resource().Attributes()) {
				actualIms := actualRms.At(i).ScopeMetrics()
				for j := 0; j < actualIms.Len(); j++ {
					actualMetricsList := actualIms.At(j).Metrics()
					transferTimeStampValues(actualMetricsList, value)
				}
			}
		}
	}
}

func transferTimeStampValues(actualMetricsList, expectedMetricsList pmetric.MetricSlice) {
	for k := 0; k < actualMetricsList.Len(); k++ {
		for l := 0; l < expectedMetricsList.Len(); l++ {
			valueMetric := expectedMetricsList.At(l)
			actualMetric := actualMetricsList.At(k)
			if valueMetric.Name() == actualMetric.Name() {
				switch actualMetric.Type() {
				case pmetric.MetricTypeGauge, pmetric.MetricTypeSum:
					actualDps := getNumberDataPointSlice(actualMetric)
					valueDps := getNumberDataPointSlice(valueMetric)
					matchedIndex := CheckNumberAttributeEquivalence(valueDps, actualDps)
					for valIndex, dpsIndex := range matchedIndex {
						actualDps.At(dpsIndex).SetTimestamp(valueDps.At(valIndex).Timestamp())
						actualDps.At(dpsIndex).SetStartTimestamp(valueDps.At(valIndex).StartTimestamp())
					}
				case pmetric.MetricTypeHistogram:
					actualDps := actualMetric.Histogram().DataPoints()
					valueDps := valueMetric.Histogram().DataPoints()
					matchedIndex := CheckHistogramAttributeEquivalence(valueDps, actualDps)
					for valIndex, dpsIndex := range matchedIndex {
						actualDps.At(dpsIndex).SetTimestamp(valueDps.At(valIndex).Timestamp())
						actualDps.At(dpsIndex).SetStartTimestamp(valueDps.At(valIndex).StartTimestamp())
					}
				case pmetric.MetricTypeExponentialHistogram:
					actualDps := actualMetric.ExponentialHistogram().DataPoints()
					valueDps := valueMetric.ExponentialHistogram().DataPoints()
					matchedIndex := CheckExponentialHistogramAttributeEquivalence(valueDps, actualDps)
					for valIndex, dpsIndex := range matchedIndex {
						actualDps.At(dpsIndex).SetTimestamp(valueDps.At(valIndex).Timestamp())
						actualDps.At(dpsIndex).SetStartTimestamp(valueDps.At(valIndex).StartTimestamp())
					}
				case pmetric.MetricTypeSummary:
					actualDps := actualMetric.Summary().DataPoints()
					valueDps := valueMetric.Summary().DataPoints()
					matchedIndex := CheckSummaryAttributeEquivalence(valueDps, actualDps)
					for valIndex, dpsIndex := range matchedIndex {
						actualDps.At(dpsIndex).SetTimestamp(valueDps.At(valIndex).Timestamp())
						actualDps.At(dpsIndex).SetStartTimestamp(valueDps.At(valIndex).StartTimestamp())
					}
				}
			}
		}
	}
}

// compares two pmetric.NumberDataPointSlices and checks if the attribute values are the same for each corresponding index
// returns a map where the key represents the index of the match for the first paramter slice
// and the val represents the index of the match of the second paramter slice
func CheckNumberAttributeEquivalence(value, dps pmetric.NumberDataPointSlice) map[int]int {
	matchedIndex := make(map[int]int)
	for i := 0; i < value.Len(); i++ {
		for j := 0; j < dps.Len(); j++ {
			if !reflect.DeepEqual(value.At(i).Attributes().AsRaw(), dps.At(j).Attributes().AsRaw()) {
				continue
			}
			matchedIndex[i] = j
		}
	}
	return matchedIndex
}

func CheckSummaryAttributeEquivalence(value, dps pmetric.SummaryDataPointSlice) map[int]int {
	matchedIndex := make(map[int]int)
	for i := 0; i < value.Len(); i++ {
		for j := 0; j < dps.Len(); j++ {
			if !reflect.DeepEqual(value.At(i).Attributes().AsRaw(), dps.At(j).Attributes().AsRaw()) {
				continue
			}
			matchedIndex[i] = j
		}
	}
	return matchedIndex
}

func CheckHistogramAttributeEquivalence(value, dps pmetric.HistogramDataPointSlice) map[int]int {
	matchedIndex := make(map[int]int)
	for i := 0; i < value.Len(); i++ {
		for j := 0; j < dps.Len(); j++ {
			if !reflect.DeepEqual(value.At(i).Attributes().AsRaw(), dps.At(j).Attributes().AsRaw()) {
				continue
			}
			matchedIndex[i] = j
		}
	}
	return matchedIndex
}

func CheckExponentialHistogramAttributeEquivalence(value, dps pmetric.ExponentialHistogramDataPointSlice) map[int]int {
	matchedIndex := make(map[int]int)
	for i := 0; i < value.Len(); i++ {
		for j := 0; j < dps.Len(); j++ {
			if !reflect.DeepEqual(value.At(i).Attributes().AsRaw(), dps.At(j).Attributes().AsRaw()) {
				continue
			}
			matchedIndex[i] = j
		}
	}
	return matchedIndex
}

func getNumberDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.Type()))
	}
	return dataPointSlice
}
