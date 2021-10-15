// Copyright  The OpenTelemetry Authors
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

package scrapertest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
)

func FileToMetrics(filePath string) (pdata.Metrics, error) {
	expectedFileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return pdata.Metrics{}, err
	}
	unmarshaller := otlp.NewJSONMetricsUnmarshaler()
	return unmarshaller.UnmarshalMetrics(expectedFileBytes)
}

func MetricsToFile(filePath string, metrics pdata.Metrics) error {
	bytes, err := otlp.NewJSONMetricsMarshaler().MarshalMetrics(metrics)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	json.Unmarshal(bytes, &jsonVal)
	b, err := json.MarshalIndent(jsonVal, "", "   ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, b, 0600)
}

// CompareMetrics compares each part of two given metric slices and returns
// an error if they don't match. The error describes what didn't match.
func CompareMetrics(expectedAll, actualAll pdata.MetricSlice) error {
	if actualAll.Len() != expectedAll.Len() {
		return fmt.Errorf("metric slices not of same length")
	}

	lessFunc := func(a, b pdata.Metric) bool {
		return a.Name() < b.Name()
	}

	actualMetrics := actualAll.Sort(lessFunc)
	expectedMetrics := expectedAll.Sort(lessFunc)

	for i := 0; i < actualMetrics.Len(); i++ {
		actual := actualMetrics.At(i)
		expected := expectedMetrics.At(i)

		if actual.Name() != expected.Name() {
			return fmt.Errorf("metric name does not match expected: %s, actual: %s", expected.Name(), actual.Name())
		}
		if actual.DataType() != expected.DataType() {
			return fmt.Errorf("metric datatype does not match expected: %s, actual: %s", expected.DataType(), actual.DataType())
		}
		if actual.Description() != expected.Description() {
			return fmt.Errorf("metric description does not match expected: %s, actual: %s", expected.Description(), actual.Description())
		}
		if actual.Unit() != expected.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", expected.Unit(), actual.Unit())
		}

		var actualDataPoints pdata.NumberDataPointSlice
		var expectedDataPoints pdata.NumberDataPointSlice

		switch actual.DataType() {
		case pdata.MetricDataTypeGauge:
			actualDataPoints = actual.Gauge().DataPoints()
			expectedDataPoints = expected.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			if actual.Sum().AggregationTemporality() != expected.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expected.Sum().AggregationTemporality(), actual.Sum().AggregationTemporality())
			}
			if actual.Sum().IsMonotonic() != expected.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", expected.Sum().IsMonotonic(), actual.Sum().IsMonotonic())
			}
			actualDataPoints = actual.Sum().DataPoints()
			expectedDataPoints = expected.Sum().DataPoints()
		}

		if actualDataPoints.Len() != expectedDataPoints.Len() {
			return fmt.Errorf("length of datapoints don't match, metric name: %s", actual.Name())
		}

		dataPointMatches := 0
		for j := 0; j < expectedDataPoints.Len(); j++ {
			edp := expectedDataPoints.At(j)
			for k := 0; k < actualDataPoints.Len(); k++ {
				adp := actualDataPoints.At(k)
				adpAttributes := adp.Attributes()
				labelMatches := true

				if edp.Attributes().Len() != adpAttributes.Len() {
					break
				}
				edp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
					if attributeVal, ok := adpAttributes.Get(k); ok && attributeVal.StringVal() == v.StringVal() {
						return true
					}
					labelMatches = false
					return false
				})
				if !labelMatches {
					continue
				}
				if edp.Type() != adp.Type() {
					return fmt.Errorf("metric datapoint types don't match: expected type: %v, actual type: %v", edp.Type(), adp.Type())
				}
				if edp.IntVal() != adp.IntVal() {
					return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", edp.IntVal(), adp.IntVal())
				}
				if edp.DoubleVal() != adp.DoubleVal() {
					return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", edp.DoubleVal(), adp.DoubleVal())
				}
				dataPointMatches++
				break
			}
		}
		if dataPointMatches != expectedDataPoints.Len() {
			return fmt.Errorf("missing datapoints for: metric name: %s", actual.Name())
		}
	}
	return nil
}
