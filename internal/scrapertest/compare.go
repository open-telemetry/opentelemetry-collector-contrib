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

package scrapertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

// CompareOption is applied by the CompareMetricSlices function
// to mutates an expected and/or actual result before comparing.
type CompareOption interface {
	apply(expected, actual pdata.MetricSlice)
}

// CompareMetricSlices compares each part of two given MetricSlices and returns
// an error if they don't match. The error describes what didn't match. The
// expected and actual values are clones before options are applied.
func CompareMetricSlices(expected, actual pdata.MetricSlice, options ...CompareOption) error {
	expected, actual = cloneMetricSlice(expected), cloneMetricSlice(actual)

	for _, option := range options {
		option.apply(expected, actual)
	}

	if actual.Len() != expected.Len() {
		return fmt.Errorf("metric slices not of same length")
	}

	expectedByName, actualByName := metricsByName(expected), metricsByName(actual)

	var errs error
	for name := range actualByName {
		_, ok := expectedByName[name]
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected metric: %s", name))
		}
	}
	for name := range expectedByName {
		if _, ok := actualByName[name]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("missing expected metric: %s", name))
		}
	}

	if errs != nil {
		return errs
	}

	for name, actualMetric := range actualByName {
		expectedMetric := expectedByName[name]
		if actualMetric.Description() != expectedMetric.Description() {
			return fmt.Errorf("metric Description does not match expected: %s, actual: %s", expectedMetric.Description(), actualMetric.Description())
		}
		if actualMetric.Unit() != expectedMetric.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", expectedMetric.Unit(), actualMetric.Unit())
		}
		if actualMetric.DataType() != expectedMetric.DataType() {
			return fmt.Errorf("metric DataType does not match expected: %s, actual: %s", expectedMetric.DataType(), actualMetric.DataType())
		}

		var expectedDataPoints pdata.NumberDataPointSlice
		var actualDataPoints pdata.NumberDataPointSlice

		switch actualMetric.DataType() {
		case pdata.MetricDataTypeGauge:
			expectedDataPoints = expectedMetric.Gauge().DataPoints()
			actualDataPoints = actualMetric.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			if actualMetric.Sum().AggregationTemporality() != expectedMetric.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expectedMetric.Sum().AggregationTemporality(), actualMetric.Sum().AggregationTemporality())
			}
			if actualMetric.Sum().IsMonotonic() != expectedMetric.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", expectedMetric.Sum().IsMonotonic(), actualMetric.Sum().IsMonotonic())
			}
			expectedDataPoints = expectedMetric.Sum().DataPoints()
			actualDataPoints = actualMetric.Sum().DataPoints()
		}

		if err := CompareNumberDataPointSlices(expectedDataPoints, actualDataPoints); err != nil {
			return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
		}
	}
	return nil
}

// CompareNumberDataPointSlices compares each part of two given NumberDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPointSlices(expected, actual pdata.NumberDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("length of datapoints don't match")
	}

	var errs error
	for j := 0; j < expected.Len(); j++ {
		edp := expected.At(j)
		var foundMatch bool
		for k := 0; k < actual.Len(); k++ {
			adp := actual.At(k)
			if reflect.DeepEqual(edp.Attributes().Sort().AsRaw(), adp.Attributes().Sort().AsRaw()) {
				foundMatch = true
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected datapoint with attributes: %v", edp.Attributes().AsRaw()))
		}
	}

	matchingDPS := make(map[pdata.NumberDataPoint]pdata.NumberDataPoint, actual.Len())
	for j := 0; j < actual.Len(); j++ {
		adp := actual.At(j)
		var foundMatch bool
		for k := 0; k < expected.Len(); k++ {
			edp := expected.At(k)
			if reflect.DeepEqual(edp.Attributes().Sort(), adp.Attributes().Sort()) {
				foundMatch = true
				matchingDPS[adp] = edp
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra datapoint with attributes: %v", adp.Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}

	for adp, edp := range matchingDPS {
		if err := CompareNumberDataPoints(edp, adp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with attributes: %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareNumberDataPoints compares each part of two given NumberDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPoints(expected, actual pdata.NumberDataPoint) error {
	if expected.Type() != actual.Type() {
		return fmt.Errorf("metric datapoint types don't match: expected type: %s, actual type: %s", numberTypeToString(expected.Type()), numberTypeToString(actual.Type()))
	}
	if expected.IntVal() != actual.IntVal() {
		return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", expected.IntVal(), actual.IntVal())
	}
	if expected.DoubleVal() != actual.DoubleVal() {
		return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", expected.DoubleVal(), actual.DoubleVal())
	}
	return nil
}

func numberTypeToString(t pdata.MetricValueType) string {
	switch t {
	case pdata.MetricValueTypeInt:
		return "int"
	case pdata.MetricValueTypeDouble:
		return "double"
	default:
		return "none"
	}
}
